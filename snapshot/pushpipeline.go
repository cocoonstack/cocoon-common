package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/sync/errgroup"

	"github.com/cocoonstack/cocoon-common/manifest"
)

const (
	// Below this, compression is not worth the mediaType churn; the entry stays raw.
	compressMinBytes = 1 << 20
	// maxChunkSizeMiB keeps ChunkSizeMiB in a sane range: beyond it the byte
	// shift can overflow, and no real deployment wants multi-GiB chunks.
	maxChunkSizeMiB = 4096
	// Default PushOptions.MemoryBudgetMiB: exactly 2×(K+1)×512 MiB — the raw and
	// compressed pools each hold workers+1 buffers, so the default budget admits
	// the default 8 workers at 512 MiB chunks. zstd encoder state (a few MiB per
	// worker at level 3) is outside the accounting.
	defaultPushMemoryBudgetMiB = 9216
)

// chunkGroup is one tar entry's descriptors: len>1 means the file is chunked.
// Workers fill slots of a fixed-size slice, so the reader must never re-slice it.
type chunkGroup struct {
	name  string
	descs []manifest.Descriptor
}

// pushPipeline is the writer strategy Push selects when compression or
// chunking is enabled: a bounded worker pool plus two buffer free-lists (raw
// and compressed), so peak memory ≈ 2×(workers+1)×chunk — kept under
// PushOptions.MemoryBudgetMiB by clamping the worker count.
type pushPipeline struct {
	pusher  *Pusher
	eg      *errgroup.Group
	enc     *zstd.Encoder
	rawBufs *bufPool
	outBufs *bufPool
	name    string
	report  func(format string, args ...any)
}

// pipelineParams sanitizes the concurrency knobs and enforces the memory
// budget: the raw and compressed pools each hold workers+1 buffers, so the
// worker count solves 2(w+1)×chunk ≤ budget. Non-positive knobs take defaults.
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

func (p *Pusher) readAndUploadEntriesPipelined(ctx context.Context, opts PushOptions, r io.Reader) (*snapshotExportConfig, map[string]manifest.SnapshotFile, []manifest.Descriptor, bool, error) {
	workers, chunkSize, err := pipelineParams(opts)
	if err != nil {
		return nil, nil, nil, false, err
	}

	var enc *zstd.Encoder
	if opts.ZstdLevel > 0 {
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
		// Stop feeding the pipeline as soon as any worker has failed.
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

		compress := enc != nil && hdr.Size >= compressMinBytes

		switch {
		case chunkSize > 0 && (compress || hdr.Size > chunkSize):
			encoded = true
			descs, chunkErr := pl.enqueueChunks(egCtx, tr, hdr, chunkSize, compress)
			if chunkErr != nil {
				readErr = chunkErr
				break readLoop
			}
			groups = append(groups, chunkGroup{name: hdr.Name, descs: descs})

		case compress:
			// Compression with chunking disabled: sequential spool-encode so a
			// multi-GiB layer never has to fit in memory.
			encoded = true
			desc, upErr := p.uploadCompressedSpool(egCtx, opts.ZstdLevel, opts.Name, hdr, tr)
			if upErr != nil {
				readErr = fmt.Errorf("upload %s: %w", hdr.Name, upErr)
				break readLoop
			}
			groups = append(groups, chunkGroup{name: hdr.Name, descs: []manifest.Descriptor{desc}})
			pl.report("  %s -> %s (%d bytes)", hdr.Name, desc.Digest, desc.Size)

		default:
			// Raw and unchunked: same shape as the v1 writer.
			desc, upErr := p.uploadTarEntry(egCtx, opts.Name, hdr, tr)
			if upErr != nil {
				readErr = fmt.Errorf("upload %s: %w", hdr.Name, upErr)
				break readLoop
			}
			groups = append(groups, chunkGroup{name: hdr.Name, descs: []manifest.Descriptor{desc}})
			pl.report("  %s -> %s (%d bytes)", hdr.Name, desc.Digest, desc.Size)
		}
	}

	waitErr := pl.eg.Wait()
	switch {
	case readErr == nil:
		readErr = waitErr
	case waitErr != nil && errors.Is(readErr, context.Canceled):
		// The reader failed because a worker's error canceled egCtx; the
		// worker error is the root cause, not the cancellation echo.
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

// enqueueChunks cuts one tar entry at fixed uncompressed offsets and hands each
// chunk to the worker pool. The returned slice is filled by workers; it is only
// complete after eg.Wait.
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
		// A failed worker cancels ctx; stop cutting the rest of the file.
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
			defer pl.rawBufs.put(buf)
			desc, upErr := pl.uploadChunk(ctx, mediaType, title, i, count, data)
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

func (pl *pushPipeline) uploadChunk(ctx context.Context, mediaType, title string, index, count int, raw []byte) (manifest.Descriptor, error) {
	stored := raw
	if manifest.IsZstdMediaType(mediaType) {
		out := pl.outBufs.take(0)
		stored = pl.enc.EncodeAll(raw, out[:0])
		defer pl.outBufs.put(stored)
	}

	sum := sha256.Sum256(stored)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	exists, existsErr := pl.pusher.Uploader.HasBlob(ctx, pl.name, digest)
	if existsErr != nil {
		return manifest.Descriptor{}, fmt.Errorf("check blob %s: %w", digest, existsErr)
	}
	if !exists {
		if err := pl.pusher.Uploader.PutBlob(ctx, pl.name, digest, bytes.NewReader(stored), int64(len(stored))); err != nil {
			return manifest.Descriptor{}, fmt.Errorf("put blob %s: %w", digest, err)
		}
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

func (p *Pusher) uploadCompressedSpool(ctx context.Context, level int, name string, hdr *tar.Header, body io.Reader) (manifest.Descriptor, error) {
	tmp, err := os.CreateTemp("", "cocoon-snapshot-*")
	if err != nil {
		return manifest.Descriptor{}, fmt.Errorf("create temp: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	h := sha256.New()
	cw := &countingWriter{w: io.MultiWriter(tmp, h)}
	enc, err := zstd.NewWriter(cw,
		zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)),
		zstd.WithEncoderConcurrency(1))
	if err != nil {
		return manifest.Descriptor{}, fmt.Errorf("init zstd encoder: %w", err)
	}
	if _, copyErr := io.Copy(enc, io.LimitReader(body, hdr.Size)); copyErr != nil {
		_ = enc.Close()
		return manifest.Descriptor{}, fmt.Errorf("compress entry: %w", copyErr)
	}
	if closeErr := enc.Close(); closeErr != nil {
		return manifest.Descriptor{}, fmt.Errorf("flush zstd encoder: %w", closeErr)
	}

	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	exists, existsErr := p.Uploader.HasBlob(ctx, name, digest)
	if existsErr != nil {
		return manifest.Descriptor{}, fmt.Errorf("check blob %s: %w", digest, existsErr)
	}
	if !exists {
		if _, seekErr := tmp.Seek(0, io.SeekStart); seekErr != nil {
			return manifest.Descriptor{}, fmt.Errorf("seek temp: %w", seekErr)
		}
		if putErr := p.Uploader.PutBlob(ctx, name, digest, tmp, cw.n); putErr != nil {
			return manifest.Descriptor{}, fmt.Errorf("put blob %s: %w", digest, putErr)
		}
	}

	return manifest.Descriptor{
		MediaType:   manifest.ZstdMediaType(manifest.MediaTypeForCocoonFile(hdr.Name)),
		Digest:      digest,
		Size:        cw.n,
		Annotations: map[string]string{manifest.AnnotationTitle: hdr.Name},
	}, nil
}

// bufPool is a fixed-capacity free-list; take blocks when all buffers are out,
// which is the pipeline's memory bound and backpressure.
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

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
