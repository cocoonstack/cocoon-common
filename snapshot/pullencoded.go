package snapshot

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/ociutil"
)

const (
	// defaultPullPrefetchBudget caps the bytes held by in-flight prefetched
	// chunks of one Stream call; StreamOptions.MemoryBudgetMiB overrides it.
	defaultPullPrefetchBudget = 4 << 30
	// maxBufferedChunkBytes bounds a single buffered chunk: anything larger —
	// including whole-file single blobs from compress-only pushes — streams
	// sequentially instead of being read into memory.
	maxBufferedChunkBytes = 1 << 30
)

// writeEncodedImportTar is the v2 assembly, selected by StreamParsed for
// ArtifactTypeSnapshotV2 manifests: layers may be zstd-compressed and/or split
// across chunk blobs; small raw layers pass through like v1. Layers were
// already validated by validateSnapshotLayers.
func writeEncodedImportTar(ctx context.Context, dl Downloader, name, localName string, cfg *manifest.SnapshotConfig, layers []manifest.Descriptor, w io.Writer, progress func(string), prefetch int, budget int64) error {
	bw := bufio.NewWriterSize(w, 256<<10)
	tw := tar.NewWriter(bw)

	now := nowFunc()
	if err := writeSnapshotEnvelope(tw, cfg, localName, now); err != nil {
		return err
	}

	byDigest := make(map[string]manifest.Descriptor, len(layers))
	for _, layer := range layers {
		byDigest[layer.Digest] = layer
	}

	emitted := map[string]bool{}
	for _, layer := range layers {
		title := layer.Title()
		var fileMeta manifest.SnapshotFile
		if cfg.Files != nil {
			fileMeta = cfg.Files[title]
		}

		raw := len(fileMeta.Chunks) == 0 && !manifest.IsZstdMediaType(layer.MediaType)
		if raw {
			if progress != nil {
				progress(fmt.Sprintf("  %s (%d bytes)", title, layer.Size))
			}
			if err := streamLayerToTar(ctx, dl, name, layer, fileMeta, tw, now); err != nil {
				return err
			}
			continue
		}

		if emitted[title] {
			continue // continuation chunk of a file already streamed
		}
		emitted[title] = true
		descs, err := resolveEncodedFile(layer, fileMeta, byDigest)
		if err != nil {
			return err
		}
		if progress != nil {
			progress(fmt.Sprintf("  %s (%d bytes, %d chunks)", title, fileMeta.Size, len(descs)))
		}
		// The decode decision comes from this file's own layer, not from the
		// resolved descriptors: a dedup winner may carry another file's mediaType.
		compressed := manifest.IsZstdMediaType(layer.MediaType)
		if err := streamEncodedFile(ctx, dl, name, title, descs, fileMeta, compressed, tw, now, prefetch, budget); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	return bw.Flush()
}

// resolveEncodedFile returns the ordered blob descriptors of one encoded file.
// SnapshotConfig.Files[].Chunks is the authoritative order; the manifest layer
// list is only a content-addressed lookup table. A resolved descriptor may be
// annotated for a different file: identical chunks dedup across files (e.g.
// zeroed 512 MiB regions shared by memory-ranges and the overlay), so title
// and mediaType annotations of the winner are irrelevant — digest+size
// verification at fetch time is the correctness gate.
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

// streamEncodedFile reconstructs one encoded file into a single tar entry.
// Chunks are independent zstd frames cut at fixed uncompressed offsets, so
// their in-order concatenation is one valid zstd stream.
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
	// Buffered prefetch only pays for itself with ≥2 chunks in the window;
	// anything else (single blobs, oversized chunks, budgets below two
	// chunks) streams with O(1) memory instead of buffering.
	window := 0
	if bufferedPrefetchOK(descs) {
		window = prefetchWindow(descs, prefetch, budget)
	}
	if window >= 2 {
		body = newChunkSource(ctx, dl, name, descs, window)
	} else {
		body = &chunkStream{ctx: ctx, dl: dl, name: name, descs: descs}
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

// prefetchWindow bounds the prefetch depth so the buffered chunks fit the
// budget, always allowing at least one in flight.
func prefetchWindow(descs []manifest.Descriptor, prefetch int, budget int64) int {
	window, held := 0, int64(0)
	for _, d := range descs {
		if window >= prefetch || (window > 0 && held+d.Size > budget) {
			break
		}
		window++
		held += d.Size
	}
	return max(window, 1)
}

type chunkFetch struct {
	data []byte
	err  error
}

// chunkSource yields chunk bodies in order with fetches running ahead;
// futures enter the queue before their fetch spawns, so buffered chunks
// (queued + the one being consumed) never exceed the window. Every chunk is
// digest- and size-verified before it is consumed.
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
			// Enqueue before spawning: a fetch blocked at the queue would
			// otherwise already be buffering, making the real hold window+1.
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

// bufferedPrefetchOK gates the parallel prefetch path: it buffers whole chunks
// in memory, so it only applies to multi-chunk files whose chunks are small
// enough to hold. Everything else streams through chunkStream.
func bufferedPrefetchOK(descs []manifest.Descriptor) bool {
	if len(descs) < 2 {
		return false
	}
	for _, d := range descs {
		if d.Size > maxBufferedChunkBytes {
			return false
		}
	}
	return true
}

// chunkStream reads chunks one at a time straight off the registry stream,
// verifying digest and size at each chunk boundary, with O(1) memory.
type chunkStream struct {
	ctx   context.Context
	dl    Downloader
	name  string
	descs []manifest.Descriptor
	i     int
	cur   io.ReadCloser
	lim   *io.LimitedReader
	hash  hash.Hash
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
			s.cur = body
			s.hash = sha256.New()
			s.lim = &io.LimitedReader{R: io.TeeReader(body, s.hash), N: desc.Size}
		}
		n, err := s.lim.Read(p)
		if err != nil && !errors.Is(err, io.EOF) {
			return n, err
		}
		if !errors.Is(err, io.EOF) {
			return n, nil
		}
		if finErr := s.finishChunk(); finErr != nil {
			return n, finErr
		}
		if n > 0 {
			return n, nil
		}
	}
}

// finishChunk enforces the same contract as ociutil.CopyBlobExact — exact
// size, no trailing bytes, digest match — at a chunk boundary.
func (s *chunkStream) finishChunk() error {
	desc := s.descs[s.i]
	var probe [1]byte
	extra, _ := s.cur.Read(probe[:])
	_ = s.cur.Close()
	if extra > 0 {
		return fmt.Errorf("blob %s longer than manifest size %d", desc.Digest, desc.Size)
	}
	if s.lim.N > 0 {
		return fmt.Errorf("blob %s shorter than manifest size %d (missing %d)", desc.Digest, desc.Size, s.lim.N)
	}
	got := "sha256:" + hex.EncodeToString(s.hash.Sum(nil))
	want := desc.Digest
	if !strings.HasPrefix(want, "sha256:") {
		want = "sha256:" + want
	}
	if got != want {
		return fmt.Errorf("blob %s digest mismatch: got %s", desc.Digest, got)
	}
	s.cur, s.lim, s.hash = nil, nil, nil
	s.i++
	return nil
}
