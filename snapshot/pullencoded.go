package snapshot

import (
	"archive/tar"
	"bufio"
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

// pullPrefetchBudget caps the bytes held by in-flight prefetched chunks; the
// window shrinks below the configured concurrency when chunks are large.
const pullPrefetchBudget = 4 << 30

// writeEncodedImportTar is the v2 assembly, selected by StreamParsed for
// ArtifactTypeSnapshotV2 manifests: layers may be zstd-compressed and/or split
// across chunk blobs; small raw layers pass through like v1. Layers were
// already validated by validateSnapshotLayers.
func writeEncodedImportTar(ctx context.Context, dl Downloader, name, localName string, cfg *manifest.SnapshotConfig, layers []manifest.Descriptor, w io.Writer, progress func(string), prefetch int) error {
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
		if err := streamEncodedFile(ctx, dl, name, title, descs, fileMeta, compressed, tw, now, prefetch); err != nil {
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
func streamEncodedFile(ctx context.Context, dl Downloader, name, title string, descs []manifest.Descriptor, fileMeta manifest.SnapshotFile, compressed bool, tw *tar.Writer, modTime time.Time, prefetch int) error {
	hdr, err := layerHeader(title, fileMeta.Size, fileMeta, modTime)
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var body io.Reader = newChunkSource(ctx, dl, name, descs, prefetchWindow(descs, prefetch))
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
func prefetchWindow(descs []manifest.Descriptor, prefetch int) int {
	window, held := 0, int64(0)
	for _, d := range descs {
		if window >= prefetch || (window > 0 && held+d.Size > pullPrefetchBudget) {
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

// chunkSource yields chunk bodies in order while up to ~window fetches run
// ahead; every chunk is digest- and size-verified before it is consumed.
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
			go func() {
				data, err := fetchChunk(ctx, dl, name, desc)
				fut <- chunkFetch{data: data, err: err}
			}()
			select {
			case futures <- fut:
			case <-ctx.Done():
				return
			}
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
