package snapshot

import (
	"archive/tar"
	"bytes"
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

// resolveEncodedFile returns one encoded file's descriptors in Files[].Chunks
// order. Identical chunks dedup across files, so a resolved descriptor may
// carry another file's annotations; digest+size verification is the gate.
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

// layerPlan is one resolved manifest layer in tar order: a whole stored layer, or
// a file reassembled from chunks when chunks is set.
type layerPlan struct {
	title  string
	meta   manifest.SnapshotFile
	layer  manifest.Descriptor
	chunks []manifest.Descriptor
	zstd   bool
}

func (e layerPlan) encoded() bool { return e.chunks != nil }

// slotBytes is what one in-flight chunk of this entry occupies. A decoding slot
// holds both halves at once — the stored bytes it reads and the uncompressed
// bytes it writes — so charging only the decoded half puts peak RSS at several
// times the budget.
func (e layerPlan) slotBytes() (unit, stride int64) {
	stored := int64(0)
	for _, d := range e.chunks {
		stored = max(stored, d.Size)
	}
	if !e.zstd {
		return stored, stored
	}
	stride = rawChunkStride(e.meta.Size, len(e.chunks))
	return stride + stored, stride
}

// chunkPipeline owns every buffer a pull may hold at once, shared across files:
// one window, one pool pair, one decoder. Buffers from out are released by
// chunkSource after the reader consumes them; in is borrowed and returned inside
// a single fetch.
type chunkPipeline struct {
	dl     Downloader
	name   string
	window int           // 0 = every file streams sequentially and no buffer is held
	stride int64         // decoded capacity to hand DecodeAll, sized for the widest file
	in     *bufPool      // stored bytes awaiting decode; nil when nothing is compressed
	out    *bufPool      // the bytes handed to the reader
	dec    *zstd.Decoder // nil when nothing is compressed
}

// newChunkPipeline sizes one pipeline against every entry, so the widest file
// decides the window and no file can push the pull past the budget.
func newChunkPipeline(dl Downloader, name string, entries []layerPlan, prefetch int, budget int64) (*chunkPipeline, error) {
	p := &chunkPipeline{dl: dl, name: name}
	var unit int64
	var anyZstd bool
	for _, e := range entries {
		if !e.encoded() {
			continue
		}
		entryUnit, entryStride := e.slotBytes()
		unit, p.stride = max(unit, entryUnit), max(p.stride, entryStride)
		anyZstd = anyZstd || e.zstd
	}
	if unit <= 0 || unit > maxBufferedChunkBytes {
		return p, nil
	}
	if window := min(int64(prefetch), budget/unit); window >= 2 {
		p.window = int(window)
	}
	if p.window == 0 {
		return p, nil
	}
	p.out = newBufPool(p.window + 1)
	if anyZstd {
		dec, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(p.window))
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

// fileWindow is the shared window narrowed to what this file can use; 0 sends it
// to the O(1) sequential path.
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

// streamFile reconstructs one file into a single tar entry; chunks are
// independent zstd frames, so their in-order concatenation is one valid stream.
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

	var body io.Reader
	if window := p.fileWindow(e); window >= 2 {
		// Chunks are independent frames, so decoding them in the fetch goroutines
		// spreads the work over the window instead of one serial stream decoder.
		body = newChunkSource(ctx, p, e, window)
	} else {
		cs := &chunkStream{ctx: ctx, dl: p.dl, name: p.name, descs: e.chunks}
		defer func() { _ = cs.Close() }()
		body = cs
		if e.zstd {
			dec, decErr := zstd.NewReader(body)
			if decErr != nil {
				return fmt.Errorf("init zstd decoder for %s: %w", e.title, decErr)
			}
			defer dec.Close()
			body = dec
		}
	}

	written, err := io.Copy(tw, body)
	if err != nil {
		return fmt.Errorf("stream %s: %w", e.title, err)
	}
	if written != e.meta.Size {
		return fmt.Errorf("%s reconstructed to %d bytes, want %d", e.title, written, e.meta.Size)
	}
	return nil
}

// rawChunkStride bounds the uncompressed stride push cut the file at: with n
// chunks totalling size, every chunk but the last is exactly the stride, so the
// stride is at most ⌈(size-1)/(n-1)⌉. The manifest records only stored sizes.
func rawChunkStride(size int64, n int) int64 {
	if n <= 1 || size <= 0 {
		return size
	}
	return (size-1)/int64(n-1) + 1
}

type chunkFetch struct {
	data []byte
	err  error
}

// chunkSource yields chunk bodies in order with fetches running ahead; futures
// enter the queue before their fetch spawns, so buffered chunks never exceed the
// window. Each yielded buffer goes back to the fetcher's pool once consumed —
// without that the per-chunk allocations become garbage the collector is free to
// pile up to twice the live set before running.
type chunkSource struct {
	futures chan chan chunkFetch
	pipe    *chunkPipeline
	cur     *bytes.Reader
	curBuf  []byte
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
				data, err := p.fetch(ctx, desc, e.zstd)
				fut <- chunkFetch{data: data, err: err}
			}()
		}
	}()
	return &chunkSource{futures: futures, pipe: p}
}

func (s *chunkSource) Read(p []byte) (int, error) {
	for {
		if s.cur != nil {
			n, err := s.cur.Read(p)
			if errors.Is(err, io.EOF) {
				s.cur = nil
				s.pipe.out.put(s.curBuf)
				s.curBuf = nil
				if n > 0 {
					return n, nil
				}
				continue
			}
			return n, err
		}
		fut, ok := <-s.futures
		if !ok {
			return 0, io.EOF
		}
		res := <-fut
		if res.err != nil {
			return 0, res.err
		}
		s.cur, s.curBuf = bytes.NewReader(res.data), res.data
	}
}

// fetch returns one chunk's bytes out of the bounded pools, so the window — not
// the garbage collector — decides how much memory a pull holds. Every borrowed
// buffer is returned on the error paths too, or the pools starve.
func (p *chunkPipeline) fetch(ctx context.Context, desc manifest.Descriptor, compressed bool) ([]byte, error) {
	if desc.Size < 0 || desc.Size > maxBufferedChunkBytes {
		return nil, fmt.Errorf("blob %s size %d outside bufferable range", desc.Digest, desc.Size)
	}
	if !compressed {
		buf := p.out.take(desc.Size)
		stored, err := p.read(ctx, desc, buf)
		if err != nil {
			p.out.put(buf)
			return nil, err
		}
		return stored, nil
	}
	comp := p.in.take(desc.Size)
	stored, err := p.read(ctx, desc, comp)
	if err != nil {
		p.in.put(comp)
		return nil, err
	}
	dst := p.out.take(p.stride)
	out, err := p.dec.DecodeAll(stored, dst[:0])
	p.in.put(comp)
	if err != nil {
		p.out.put(dst)
		return nil, fmt.Errorf("decode chunk %s: %w", desc.Digest, err)
	}
	return out, nil
}

// read fills buf[:desc.Size] with the blob. Exact-size read rather than
// bytes.Buffer.ReadFrom: its 512-byte grow step overshoots a pre-sized capacity
// near EOF and then reallocates and copies the whole chunk, which measured 8% of
// pull CPU.
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
	if _, err = io.Copy(io.Discard, v); err != nil {
		return nil, err
	}
	return stored, nil
}

// chunkStream reads chunks one at a time straight off the registry stream with
// O(1) memory; BlobVerifier returns io.EOF only after the blob checks pass.
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
