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

// streamEncodedFile reconstructs one file into a single tar entry; chunks are
// independent zstd frames, so their in-order concatenation is one valid stream.
func streamEncodedFile(ctx context.Context, dl Downloader, name, title string, descs []manifest.Descriptor, fileMeta manifest.SnapshotFile, compressed bool, tw *tar.Writer, modTime time.Time, prefetch int, budget int64) error {
	hdr, err := layerHeader(title, fileMeta.Size, fileMeta, modTime)
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var body io.Reader
	if window := prefetchWindow(descs, prefetch, budget); window >= 2 {
		body = newChunkSource(ctx, dl, name, descs, window)
	} else {
		cs := &chunkStream{ctx: ctx, dl: dl, name: name, descs: descs}
		defer func() { _ = cs.Close() }()
		body = cs
	}
	if compressed {
		dec, decErr := zstd.NewReader(body)
		if decErr != nil {
			return fmt.Errorf("init zstd decoder for %s: %w", title, decErr)
		}
		defer dec.Close()
		body = dec
	}

	written, err := io.Copy(tw, body)
	if err != nil {
		return fmt.Errorf("stream %s: %w", title, err)
	}
	if written != fileMeta.Size {
		return fmt.Errorf("%s reconstructed to %d bytes, want %d", title, written, fileMeta.Size)
	}
	return nil
}

// prefetchWindow sizes the buffered window by the file's largest chunk (sizes
// are heterogeneous) so window×max ≤ budget holds at every point of the
// stream; 0 sends the caller to the O(1) sequential path.
func prefetchWindow(descs []manifest.Descriptor, prefetch int, budget int64) int {
	if len(descs) < 2 {
		return 0
	}
	var maxSize int64
	for _, d := range descs {
		if d.Size > maxBufferedChunkBytes {
			return 0
		}
		maxSize = max(maxSize, d.Size)
	}
	if maxSize <= 0 {
		return 0
	}
	window := min(int64(prefetch), budget/maxSize, int64(len(descs)))
	if window < 2 {
		return 0
	}
	return int(window)
}

type chunkFetch struct {
	data []byte
	err  error
}

// chunkSource yields verified chunk bodies in order with fetches running
// ahead; futures enter the queue before their fetch spawns, so buffered
// chunks never exceed the window.
type chunkSource struct {
	futures chan chan chunkFetch
	cur     *bytes.Reader
}

func newChunkSource(ctx context.Context, dl Downloader, name string, descs []manifest.Descriptor, window int) *chunkSource {
	futures := make(chan chan chunkFetch, max(window-1, 0))
	go func() {
		defer close(futures)
		for _, desc := range descs {
			fut := make(chan chunkFetch, 1)
			select {
			case futures <- fut:
			case <-ctx.Done():
				return
			}
			go func() {
				data, err := fetchChunk(ctx, dl, name, desc)
				fut <- chunkFetch{data: data, err: err}
			}()
		}
	}()
	return &chunkSource{futures: futures}
}

func (s *chunkSource) Read(p []byte) (int, error) {
	for {
		if s.cur != nil {
			n, err := s.cur.Read(p)
			if errors.Is(err, io.EOF) {
				s.cur = nil
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
		s.cur = bytes.NewReader(res.data)
	}
}

func fetchChunk(ctx context.Context, dl Downloader, name string, desc manifest.Descriptor) ([]byte, error) {
	if desc.Size < 0 || desc.Size > maxBufferedChunkBytes {
		return nil, fmt.Errorf("blob %s size %d outside bufferable range", desc.Digest, desc.Size)
	}
	body, err := dl.GetBlob(ctx, name, desc.Digest)
	if err != nil {
		return nil, fmt.Errorf("get blob %s: %w", desc.Digest, err)
	}
	defer func() { _ = body.Close() }()
	buf := bytes.NewBuffer(make([]byte, 0, desc.Size))
	if err := ociutil.CopyBlobExact(buf, body, desc.Digest, desc.Size); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
			s.cur = ociutil.NewBlobVerifier(body, desc.Digest, desc.Size)
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
