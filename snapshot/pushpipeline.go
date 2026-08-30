package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/sync/errgroup"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/ociutil"
)

const (
	// Below this, compression is not worth the mediaType churn.
	compressMinBytes = 1 << 20
	// Beyond this the MiB→bytes shift could overflow.
	maxChunkSizeMiB = 4096
	// 2 pools × (defaultTransferConcurrency+1) buffers × 512 MiB chunks.
	defaultPushMemoryBudgetMiB = 9216
)

type chunkGroup struct {
	name  string
	descs []manifest.Descriptor
}

// pushPipeline is the encode/upload worker pool Push selects when compression or chunking is on.
type pushPipeline struct {
	pusher  *Pusher
	eg      *errgroup.Group
	enc     *zstd.Encoder
	rawBufs *bufPool
	outBufs *bufPool
	name    string
	report  func(format string, args ...any)
}

// enqueueChunks cuts one tar entry at fixed offsets into the worker pool; the returned slice is complete only after eg.Wait.
func (pl *pushPipeline) enqueueChunks(ctx context.Context, tr *tar.Reader, hdr *tar.Header, chunkSize int64, compress bool) ([]manifest.Descriptor, error) {
	count64 := (hdr.Size + chunkSize - 1) / chunkSize
	if count64 > 1<<20 {
		return nil, fmt.Errorf("%s: %d chunks exceeds sanity cap", hdr.Name, count64)
	}
	count := int(count64)
	mediaType := manifest.MediaTypeForCocoonFile(hdr.Name)
	if compress {
		mediaType = manifest.ZstdMediaType(mediaType)
	}

	title := hdr.Name
	descs := make([]manifest.Descriptor, count)
	remaining := hdr.Size
	for i := range count {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		want := min(chunkSize, remaining)
		remaining -= want
		buf := pl.rawBufs.take(chunkSize)
		data := buf[:want]
		if _, readFullErr := io.ReadFull(tr, data); readFullErr != nil {
			pl.rawBufs.put(buf)
			return nil, fmt.Errorf("read %s chunk %d: %w", title, i, readFullErr)
		}
		pl.eg.Go(func() error {
			desc, upErr := pl.uploadChunk(ctx, mediaType, title, i, count, data, buf)
			if upErr != nil {
				return fmt.Errorf("upload %s chunk %d: %w", title, i, upErr)
			}
			descs[i] = desc
			pl.report("  %s [%d/%d] -> %s (%d bytes)", title, i+1, count, desc.Digest, desc.Size)
			return nil
		})
	}
	return descs, nil
}

// uploadChunk owns rawBuf: returned after EncodeAll when compressing, after upload when raw.
func (pl *pushPipeline) uploadChunk(ctx context.Context, mediaType, title string, index, count int, raw, rawBuf []byte) (manifest.Descriptor, error) {
	stored := raw
	if manifest.IsZstdMediaType(mediaType) {
		out := pl.outBufs.take(int64(len(raw)))
		stored = pl.enc.EncodeAll(raw, out[:0])
		pl.rawBufs.put(rawBuf)
		defer pl.outBufs.put(stored)
	} else {
		defer pl.rawBufs.put(rawBuf)
	}

	digest := "sha256:" + ociutil.SHA256Hex(stored)
	if err := pl.pusher.putBlobIfMissing(ctx, pl.name, digest, bytes.NewReader(stored), int64(len(stored))); err != nil {
		return manifest.Descriptor{}, err
	}

	desc := manifest.Descriptor{
		MediaType:   mediaType,
		Digest:      digest,
		Size:        int64(len(stored)),
		Annotations: map[string]string{manifest.AnnotationTitle: title},
	}
	if count > 1 {
		desc.Annotations[manifest.AnnotationChunkIndex] = strconv.Itoa(index)
		desc.Annotations[manifest.AnnotationChunkCount] = strconv.Itoa(count)
	}
	return desc, nil
}

func (p *Pusher) readAndUploadEntriesPipelined(ctx context.Context, opts PushOptions, r io.Reader) (*snapshotExportConfig, map[string]manifest.SnapshotFile, []manifest.Descriptor, bool, error) {
	workers, chunkSize, err := pipelineParams(opts)
	if err != nil {
		return nil, nil, nil, false, err
	}

	compressOn := opts.ZstdLevel > 0
	var enc *zstd.Encoder
	if compressOn && chunkSize > 0 {
		var encErr error
		enc, encErr = zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(opts.ZstdLevel)),
			zstd.WithEncoderConcurrency(workers))
		if encErr != nil {
			return nil, nil, nil, false, fmt.Errorf("init zstd encoder: %w", encErr)
		}
		defer func() { _ = enc.Close() }()
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(workers)

	var progressMu sync.Mutex
	pl := &pushPipeline{
		pusher:  p,
		eg:      eg,
		enc:     enc,
		rawBufs: newBufPool(workers + 1),
		outBufs: newBufPool(workers + 1),
		name:    opts.Name,
		report: func(format string, args ...any) {
			if opts.Progress == nil {
				return
			}
			progressMu.Lock()
			defer progressMu.Unlock()
			opts.Progress(fmt.Sprintf(format, args...))
		},
	}

	var (
		tr      = tar.NewReader(r)
		cfg     *snapshotExportConfig
		files   = map[string]manifest.SnapshotFile{}
		groups  []chunkGroup
		encoded bool
		readErr error
	)

readLoop:
	for {
		select {
		case <-egCtx.Done():
			break readLoop
		default:
		}

		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			readErr = fmt.Errorf("read tar entry: %w", err)
			break
		}

		if hdr.Name == snapshotJSONName {
			var envelope snapshotExportEnvelope
			if decErr := json.NewDecoder(tr).Decode(&envelope); decErr != nil {
				readErr = fmt.Errorf("parse snapshot.json: %w", decErr)
				break
			}
			cfg = &envelope.Config
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if hdr.Size < 0 {
			readErr = fmt.Errorf("tar entry %s has negative size %d", hdr.Name, hdr.Size)
			break
		}

		fileMeta, metaErr := sparseFileMeta(hdr)
		if metaErr != nil {
			readErr = metaErr
			break
		}
		fileMeta.Size = hdr.Size
		files[hdr.Name] = fileMeta

		compress := compressOn && hdr.Size >= compressMinBytes
		if chunkSize > 0 && (compress || hdr.Size > chunkSize) {
			encoded = true
			descs, chunkErr := pl.enqueueChunks(egCtx, tr, hdr, chunkSize, compress)
			if chunkErr != nil {
				readErr = chunkErr
				break
			}
			groups = append(groups, chunkGroup{name: hdr.Name, descs: descs})
			continue
		}

		var desc manifest.Descriptor
		var upErr error
		if compress {
			// Chunking off: spool-encode sequentially so a multi-GiB layer never sits in memory.
			encoded = true
			desc, upErr = p.uploadCompressedSpool(egCtx, opts.ZstdLevel, opts.Name, hdr, tr)
		} else {
			desc, upErr = p.uploadTarEntry(egCtx, opts.Name, hdr, tr)
		}
		if upErr != nil {
			readErr = fmt.Errorf("upload %s: %w", hdr.Name, upErr)
			break
		}
		groups = append(groups, chunkGroup{name: hdr.Name, descs: []manifest.Descriptor{desc}})
		pl.report("  %s -> %s (%d bytes)", hdr.Name, desc.Digest, desc.Size)
	}

	waitErr := pl.eg.Wait()
	switch {
	case readErr == nil:
		readErr = waitErr
	case waitErr != nil && errors.Is(readErr, context.Canceled):
		// A worker's failure canceled egCtx; the worker error is the root cause.
		readErr = waitErr
	}
	if readErr == nil {
		readErr = ctx.Err()
	}
	if readErr != nil {
		return nil, nil, nil, false, readErr
	}

	var layers []manifest.Descriptor
	for _, group := range groups {
		layers = append(layers, group.descs...)
		if len(group.descs) < 2 {
			continue
		}
		fm := files[group.name]
		fm.Chunks = make([]string, len(group.descs))
		for i, desc := range group.descs {
			fm.Chunks[i] = desc.Digest
		}
		files[group.name] = fm
	}
	return cfg, files, layers, encoded, nil
}

func (p *Pusher) uploadCompressedSpool(ctx context.Context, level int, name string, hdr *tar.Header, body io.Reader) (manifest.Descriptor, error) {
	return p.uploadSpooled(ctx, name, hdr.Name, manifest.ZstdMediaType(manifest.MediaTypeForCocoonFile(hdr.Name)), func(w io.Writer) error {
		enc, err := zstd.NewWriter(w,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)),
			zstd.WithEncoderConcurrency(1))
		if err != nil {
			return fmt.Errorf("init zstd encoder: %w", err)
		}
		if _, err := io.Copy(enc, io.LimitReader(body, hdr.Size)); err != nil {
			_ = enc.Close()
			return fmt.Errorf("compress entry: %w", err)
		}
		if err := enc.Close(); err != nil {
			return fmt.Errorf("flush zstd encoder: %w", err)
		}
		return nil
	})
}

// pipelineParams solves 2(workers+1)×chunk ≤ budget, the combined size of both buffer pools.
func pipelineParams(opts PushOptions) (int, int64, error) {
	workers := opts.Concurrency
	if workers <= 0 {
		workers = defaultTransferConcurrency
	}
	if opts.ChunkSizeMiB > maxChunkSizeMiB {
		return 0, 0, fmt.Errorf("snapshot push: chunk size %d MiB exceeds the %d MiB maximum", opts.ChunkSizeMiB, maxChunkSizeMiB)
	}
	chunkSize := int64(max(opts.ChunkSizeMiB, 0)) << 20
	if chunkSize == 0 {
		return workers, 0, nil
	}
	budget := int64(opts.MemoryBudgetMiB) << 20
	if budget <= 0 {
		budget = defaultPushMemoryBudgetMiB << 20
	}
	// One worker already needs 2 pools × 2 buffers: 4×chunk is the floor.
	if 4*chunkSize > budget {
		return 0, 0, fmt.Errorf(
			"snapshot push: chunk size %d MiB needs at least a %d MiB memory budget, got %d MiB",
			opts.ChunkSizeMiB, (4*chunkSize)>>20, budget>>20,
		)
	}
	return min(workers, int(budget/(2*chunkSize))-1), chunkSize, nil
}

// bufPool is a fixed-capacity free-list; a blocked take is the pipeline's memory bound.
type bufPool struct {
	ch chan []byte
}

func newBufPool(size int) *bufPool {
	bp := &bufPool{ch: make(chan []byte, size)}
	for range size {
		bp.ch <- nil
	}
	return bp
}

func (bp *bufPool) take(capacity int64) []byte {
	buf := <-bp.ch
	if int64(cap(buf)) < capacity {
		buf = make([]byte, capacity)
	}
	return buf
}

func (bp *bufPool) put(buf []byte) {
	bp.ch <- buf
}
