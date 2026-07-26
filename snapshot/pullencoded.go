package snapshot

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/ociutil"
)

const (
	// Prefetch-buffer cap for one Stream call; StreamOptions.MemoryBudgetMiB overrides.
	defaultPullPrefetchBudget = 4 << 30
	// Chunks larger than this stream sequentially instead of being buffered.
	maxBufferedChunkBytes = 1 << 30
)

type layerPlan struct {
	title  string
	meta   manifest.SnapshotFile
	layer  manifest.Descriptor
	chunks []manifest.Descriptor
}

func (e layerPlan) encoded() bool { return e.chunks != nil }

func (e layerPlan) zstd() bool { return manifest.IsZstdMediaType(e.layer.MediaType) }

func (e layerPlan) bufferCaps() (input, output int64) {
	stored := int64(0)
	for _, d := range e.chunks {
		stored = max(stored, d.Size)
	}
	if !e.zstd() {
		return 0, stored
	}
	return stored, rawChunkStride(e.meta.Size, len(e.chunks))
}

type chunkPipeline struct {
	dl        Downloader
	name      string
	window    int
	inputCap  int64
	outputCap int64
	in        *bufPool
	out       *bufPool
	dec       *zstd.Decoder
}

func newChunkPipeline(dl Downloader, name string, entries []layerPlan, prefetch int, budget int64) (*chunkPipeline, error) {
	p := &chunkPipeline{dl: dl, name: name}
	var anyZstd bool
	for _, e := range entries {
		if !e.encoded() {
			continue
		}
		inputCap, outputCap := e.bufferCaps()
		p.inputCap = max(p.inputCap, inputCap)
		p.outputCap = max(p.outputCap, outputCap)
		anyZstd = anyZstd || e.zstd()
	}
	if p.outputCap <= 0 || p.inputCap > maxBufferedChunkBytes || p.outputCap > maxBufferedChunkBytes {
		return p, nil
	}
	unit := p.inputCap + p.outputCap
	if budget > p.outputCap {
		if window := min(int64(prefetch), (budget-p.outputCap)/unit); window >= 2 {
			p.window = int(window)
		}
	}
	if p.window == 0 {
		return p, nil
	}
	p.out = newBufPool(p.window + 1)
	if anyZstd {
		dec, err := zstd.NewReader(nil,
			zstd.WithDecoderConcurrency(p.window),
			zstd.WithDecoderMaxMemory(uint64(p.outputCap)),
			zstd.WithDecodeAllCapLimit(true))
		if err != nil {
			return nil, fmt.Errorf("init zstd decoder: %w", err)
		}
		p.dec, p.in = dec, newBufPool(p.window)
	}
	return p, nil
}

func (p *chunkPipeline) Close() {
	if p.dec != nil {
		p.dec.Close()
	}
}

func (p *chunkPipeline) fileWindow(e layerPlan) int {
	if p.window < 2 || len(e.chunks) < 2 {
		return 0
	}
	for _, d := range e.chunks {
		if d.Size > maxBufferedChunkBytes {
			return 0
		}
	}
	return min(p.window, len(e.chunks))
}

func (p *chunkPipeline) streamFile(ctx context.Context, tw *tar.Writer, e layerPlan, modTime time.Time) error {
	hdr, err := layerHeader(e.title, e.meta.Size, e.meta, modTime)
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var written int64
	if window := p.fileWindow(e); window >= 2 {
		written, err = newChunkSource(ctx, p, e, window).WriteTo(tw)
	} else {
		cs := &chunkStream{ctx: ctx, dl: p.dl, name: p.name, descs: e.chunks}
		defer func() { _ = cs.Close() }()
		var body io.Reader = cs
		if e.zstd() {
			dec, decErr := zstd.NewReader(body)
			if decErr != nil {
				return fmt.Errorf("init zstd decoder for %s: %w", e.title, decErr)
			}
			defer dec.Close()
			body = dec
		}
		written, err = io.Copy(tw, body)
	}
	if err != nil {
		return fmt.Errorf("stream %s: %w", e.title, err)
	}
	if written != e.meta.Size {
		return fmt.Errorf("%s reconstructed to %d bytes, want %d", e.title, written, e.meta.Size)
	}
	return nil
}

func (p *chunkPipeline) fetch(ctx context.Context, desc manifest.Descriptor, compressed bool) chunkFetch {
	if desc.Size < 0 || desc.Size > maxBufferedChunkBytes {
		return chunkFetch{err: fmt.Errorf("blob %s size %d outside bufferable range", desc.Digest, desc.Size)}
	}
	if !compressed {
		buf := p.out.take(p.outputCap)
		stored, err := p.read(ctx, desc, buf)
		if err != nil {
			p.out.put(buf)
			return chunkFetch{err: err}
		}
		return chunkFetch{data: stored, buf: buf}
	}
	comp := p.in.take(p.inputCap)
	stored, err := p.read(ctx, desc, comp)
	if err != nil {
		p.in.put(comp)
		return chunkFetch{err: err}
	}
	dst := p.out.take(p.outputCap)
	out, err := p.dec.DecodeAll(stored, dst[:0])
	p.in.put(comp)
	if err != nil {
		p.out.put(dst)
		return chunkFetch{err: fmt.Errorf("decode chunk %s: %w", desc.Digest, err)}
	}
	return chunkFetch{data: out, buf: dst}
}

func (p *chunkPipeline) read(ctx context.Context, desc manifest.Descriptor, buf []byte) ([]byte, error) {
	body, err := p.dl.GetBlob(ctx, p.name, desc.Digest)
	if err != nil {
		return nil, fmt.Errorf("get blob %s: %w", desc.Digest, err)
	}
	defer func() { _ = body.Close() }()
	stored := buf[:desc.Size]
	v := ociutil.NewBlobSizeChecker(body, desc.Digest, desc.Size)
	if _, err = io.ReadFull(v, stored); err != nil {
		return nil, err
	}
	// Drain the checker: its trailing-data and transport checks run at EOF.
	if _, err = io.Copy(io.Discard, v); err != nil {
		return nil, err
	}
	return stored, nil
}

type chunkFetch struct {
	data []byte
	buf  []byte
	err  error
}

type chunkSource struct {
	futures chan chan chunkFetch
	pipe    *chunkPipeline
}

func newChunkSource(ctx context.Context, p *chunkPipeline, e layerPlan, window int) *chunkSource {
	futures := make(chan chan chunkFetch, max(window-1, 0))
	go func() {
		defer close(futures)
		for _, desc := range e.chunks {
			fut := make(chan chunkFetch, 1)
			select {
			case futures <- fut:
			case <-ctx.Done():
				return
			}
			go func() {
				fut <- p.fetch(ctx, desc, e.zstd())
			}()
		}
	}()
	return &chunkSource{futures: futures, pipe: p}
}

func (s *chunkSource) WriteTo(w io.Writer) (int64, error) {
	var written int64
	for fut := range s.futures {
		res := <-fut
		if res.err != nil {
			return written, res.err
		}
		n, err := w.Write(res.data)
		written += int64(n)
		s.pipe.out.put(res.buf)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

type chunkStream struct {
	ctx   context.Context
	dl    Downloader
	name  string
	descs []manifest.Descriptor
	i     int
	body  io.ReadCloser
	cur   io.Reader
}

func (s *chunkStream) Read(p []byte) (int, error) {
	for {
		if s.cur == nil {
			if s.i >= len(s.descs) {
				return 0, io.EOF
			}
			desc := s.descs[s.i]
			body, err := s.dl.GetBlob(s.ctx, s.name, desc.Digest)
			if err != nil {
				return 0, fmt.Errorf("get blob %s: %w", desc.Digest, err)
			}
			s.body = body
			s.cur = ociutil.NewBlobSizeChecker(body, desc.Digest, desc.Size)
		}
		n, err := s.cur.Read(p)
		if errors.Is(err, io.EOF) {
			_ = s.body.Close()
			s.cur, s.body = nil, nil
			s.i++
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

// Close releases the in-flight blob body after an aborted read.
func (s *chunkStream) Close() error {
	if s.body != nil {
		_ = s.body.Close()
		s.cur, s.body = nil, nil
	}
	return nil
}

func resolveEncodedFile(layer manifest.Descriptor, fileMeta manifest.SnapshotFile, byDigest map[string]manifest.Descriptor) ([]manifest.Descriptor, error) {
	title := layer.Title()
	if fileMeta.Size <= 0 {
		return nil, fmt.Errorf("%s: encoded layer missing files[].size in snapshot config", title)
	}
	if len(fileMeta.Chunks) == 0 {
		return []manifest.Descriptor{layer}, nil
	}
	descs := make([]manifest.Descriptor, len(fileMeta.Chunks))
	for i, digest := range fileMeta.Chunks {
		desc, ok := byDigest[digest]
		if !ok {
			return nil, fmt.Errorf("%s chunk %d (%s) missing from manifest layers", title, i, digest)
		}
		descs[i] = desc
	}
	return descs, nil
}

func rawChunkStride(size int64, n int) int64 {
	if n <= 1 || size <= 0 {
		return size
	}
	return (size-1)/int64(n-1) + 1
}
