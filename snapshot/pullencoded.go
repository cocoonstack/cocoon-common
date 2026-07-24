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
		if err := streamEncodedFile(ctx, dl, name, title, descs, fileMeta, tw, now, prefetch); err != nil {
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
// list is only a lookup table.
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
		if desc.Title() != title {
			return nil, fmt.Errorf("%s chunk %d (%s) is annotated for %q", title, i, digest, desc.Title())
		}
		if desc.MediaType != layer.MediaType {
			return nil, fmt.Errorf("%s chunk %d mediaType %q differs from %q", title, i, desc.MediaType, layer.MediaType)
		}
		descs[i] = desc
	}
	return descs, nil
}

// streamEncodedFile reconstructs one encoded file into a single tar entry.
// Chunks are independent zstd frames cut at fixed uncompressed offsets, so
// their in-order concatenation is one valid zstd stream.
func streamEncodedFile(ctx context.Context, dl Downloader, name, title string, descs []manifest.Descriptor, fileMeta manifest.SnapshotFile, tw *tar.Writer, modTime time.Time, prefetch int) error {
	hdr, err := layerHeader(title, fileMeta.Size, fileMeta, modTime)
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var body io.Reader = newChunkSource(ctx, dl, name, descs, prefetch)
	if manifest.IsZstdMediaType(descs[0].MediaType) {
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
