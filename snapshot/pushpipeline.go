package snapshot

import (
	"archive/tar"
	"bytes"
	"cmp"
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

// Below this, compression is not worth the mediaType churn; the entry stays raw.
const compressMinBytes = 1 << 20

// chunkGroup is one tar entry's descriptors: len>1 means the file is chunked.
// Workers fill slots of a fixed-size slice, so the reader must never re-slice it.
type chunkGroup struct {
	name  string
	descs []manifest.Descriptor
}

// pushPipeline is the writer strategy Push selects when compression or
// chunking is enabled: a bounded worker pool plus
// buffer free-lists that cap memory at ~(workers+1) chunk buffers.
type pushPipeline struct {
	pusher  *Pusher
	eg      *errgroup.Group
	ctx     context.Context
	enc     *zstd.Encoder
	rawBufs *bufPool
	outBufs *bufPool
	name    string
	report  func(format string, args ...any)
}

func (p *Pusher) readAndUploadEntriesPipelined(ctx context.Context, opts PushOptions, r io.Reader) (*snapshotExportConfig, map[string]manifest.SnapshotFile, []manifest.Descriptor, bool, error) {
	workers := cmp.Or(opts.Concurrency, defaultTransferConcurrency)
	chunkSize := int64(opts.ChunkSizeMiB) << 20

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
		ctx:     egCtx,
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
			descs, chunkErr := pl.enqueueChunks(tr, hdr, chunkSize, compress)
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
	if readErr == nil {
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
func (pl *pushPipeline) enqueueChunks(tr *tar.Reader, hdr *tar.Header, chunkSize int64, compress bool) ([]manifest.Descriptor, error) {
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
			desc, upErr := pl.uploadChunk(mediaType, title, i, count, data)
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

func (pl *pushPipeline) uploadChunk(mediaType, title string, index, count int, raw []byte) (manifest.Descriptor, error) {
	stored := raw
	if manifest.IsZstdMediaType(mediaType) {
		out := pl.outBufs.take(0)
		stored = pl.enc.EncodeAll(raw, out[:0])
		defer pl.outBufs.put(stored)
	}

	sum := sha256.Sum256(stored)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	exists, existsErr := pl.pusher.Uploader.HasBlob(pl.ctx, pl.name, digest)
	if existsErr != nil {
		return manifest.Descriptor{}, fmt.Errorf("check blob %s: %w", digest, existsErr)
	}
	if !exists {
		if err := pl.pusher.Uploader.PutBlob(pl.ctx, pl.name, digest, bytes.NewReader(stored), int64(len(stored))); err != nil {
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
	p := &bufPool{ch: make(chan []byte, size)}
	for range size {
		p.ch <- nil
	}
	return p
}

func (p *bufPool) take(capacity int64) []byte {
	buf := <-p.ch
	if int64(cap(buf)) < capacity {
		buf = make([]byte, capacity)
	}
	return buf
}

func (p *bufPool) put(buf []byte) {
	p.ch <- buf
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
