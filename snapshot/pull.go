package snapshot

import (
	"archive/tar"
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/ociutil"
)

// Real configs hit ~1.4 MB on fragmented Windows VMs; 64 MiB leaves headroom while bounding pathological reads.
const maxSnapshotConfigSize = 64 << 20

// StreamOptions configures snapshot tar stream assembly.
type StreamOptions struct {
	Name        string
	LocalName   string // empty = use Name
	Writer      io.Writer
	Progress    func(string)
	Concurrency int // parallel chunk prefetch for encoded layers (default 8)
	// MemoryBudgetMiB caps the bytes buffered by the prefetch window of one
	// Stream call (default 4096). Callers running concurrent pulls should
	// divide their node budget across them.
	MemoryBudgetMiB int
}

// Stream reassembles a snapshot manifest into a cocoon-import tar stream.
// If raw is an OCI image-index (multi-platform), a child manifest is resolved
// via dl.GetManifest(..., childDigest) and streamed instead — preferring
// linux/amd64, falling back to the first non-attestation entry.
func Stream(ctx context.Context, raw []byte, dl Downloader, opts StreamOptions) error {
	if opts.Name == "" {
		return errors.New("snapshot stream: name is required")
	}
	if opts.Writer == nil {
		return errors.New("snapshot stream: writer is required")
	}

	m, err := manifest.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.ClassifyParsed(m) == manifest.KindImageIndex {
		child, err := pickIndexChild(ctx, m)
		if err != nil {
			return err
		}
		childRaw, _, err := dl.GetManifest(ctx, opts.Name, child.Digest)
		if err != nil {
			return fmt.Errorf("get child manifest %s: %w", child.Digest, err)
		}
		m, err = manifest.Parse(childRaw)
		if err != nil {
			return fmt.Errorf("parse child manifest: %w", err)
		}
	}

	if kind := manifest.ClassifyParsed(m); kind != manifest.KindSnapshot {
		return fmt.Errorf("manifest is %s, not a snapshot", kind)
	}

	return StreamParsed(ctx, m, dl, opts)
}

// StreamParsed accepts an already-parsed manifest and dispatches on its
// artifactType: v1 manifests take the original raw-blob path, v2 manifests the
// encoded (zstd/chunked) path in pullencoded.go.
func StreamParsed(ctx context.Context, m *manifest.OCIManifest, dl Downloader, opts StreamOptions) error {
	if opts.Name == "" {
		return errors.New("snapshot stream: name is required")
	}
	if opts.Writer == nil {
		return errors.New("snapshot stream: writer is required")
	}
	localName := cmp.Or(opts.LocalName, opts.Name)

	cfg, err := FetchSnapshotConfig(ctx, dl, opts.Name, m.Config)
	if err != nil {
		return fmt.Errorf("fetch snapshot config: %w", err)
	}
	if err := validateSnapshotLayers(m, cfg); err != nil {
		return err
	}

	switch m.ArtifactType {
	case manifest.ArtifactTypeSnapshotV2:
		prefetch := opts.Concurrency
		if prefetch <= 0 {
			prefetch = defaultTransferConcurrency
		}
		budget := int64(opts.MemoryBudgetMiB) << 20
		if budget <= 0 {
			budget = defaultPullPrefetchBudget
		}
		return writeEncodedImportTar(ctx, dl, opts.Name, localName, cfg, m.Layers, opts.Writer, opts.Progress, prefetch, budget)
	default:
		return writeImportTar(ctx, dl, opts.Name, localName, cfg, m.Layers, opts.Writer, opts.Progress)
	}
}

// validateSnapshotLayers fails closed before any byte is streamed: every layer
// must be decodable by this reader, and encoded layers may only appear in
// manifests marked ArtifactTypeSnapshotV2. The alternative to failing here is
// assembling a corrupt import tar that only fails at restore time.
func validateSnapshotLayers(m *manifest.OCIManifest, cfg *manifest.SnapshotConfig) error {
	encoded := m.ArtifactType == manifest.ArtifactTypeSnapshotV2
	for _, layer := range m.Layers {
		if layer.Size < 0 {
			return fmt.Errorf("layer %s has negative size %d", layer.Digest, layer.Size)
		}
		if !manifest.IsSnapshotLayerMediaType(layer.MediaType) {
			return fmt.Errorf("layer %s has unsupported mediaType %q (snapshot needs a newer reader)", layer.Digest, layer.MediaType)
		}
		if layer.Title() == "" {
			return fmt.Errorf("layer %s missing %s annotation", layer.Digest, manifest.AnnotationTitle)
		}
		if !encoded && manifest.IsZstdMediaType(layer.MediaType) {
			return fmt.Errorf("layer %s is zstd-compressed but manifest is not %s", layer.Digest, manifest.ArtifactTypeSnapshotV2)
		}
	}
	if !encoded {
		for name, f := range cfg.Files {
			if len(f.Chunks) > 0 {
				return fmt.Errorf("file %s is chunked but manifest is not %s", name, manifest.ArtifactTypeSnapshotV2)
			}
		}
	}
	return nil
}

// FetchSnapshotConfig downloads and parses the snapshot config blob.
func FetchSnapshotConfig(ctx context.Context, dl Downloader, name string, desc manifest.Descriptor) (*manifest.SnapshotConfig, error) {
	if desc.MediaType != manifest.MediaTypeSnapshotConfig {
		return nil, fmt.Errorf("unexpected config mediaType %q", desc.MediaType)
	}
	if desc.Size > maxSnapshotConfigSize {
		return nil, fmt.Errorf("config blob too large: %d > %d", desc.Size, maxSnapshotConfigSize)
	}
	body, err := dl.GetBlob(ctx, name, desc.Digest)
	if err != nil {
		return nil, fmt.Errorf("get config blob %s: %w", desc.Digest, err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, maxSnapshotConfigSize+1))
	if err != nil {
		return nil, fmt.Errorf("read config blob: %w", err)
	}
	if int64(len(data)) > maxSnapshotConfigSize {
		return nil, fmt.Errorf("config blob exceeded cap %d while streaming", maxSnapshotConfigSize)
	}
	var cfg manifest.SnapshotConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse snapshot config: %w", err)
	}
	return &cfg, nil
}

// pickIndexChild selects the linux/amd64 child from an OCI image-index and
// falls back to the first non-attestation entry when no platform matches.
// A fallback selection is logged at Warn so operators can see which child
// was actually streamed instead of silently shipping a non-amd64 snapshot.
func pickIndexChild(ctx context.Context, m *manifest.OCIManifest) (manifest.IndexManifest, error) {
	var fallback *manifest.IndexManifest
	for i := range m.Manifests {
		c := m.Manifests[i]
		if c.Platform != nil && c.Platform.OS == "linux" && c.Platform.Architecture == "amd64" {
			return c, nil
		}
		if fallback == nil && c.Platform != nil && c.Platform.Architecture != "unknown" {
			fallback = &m.Manifests[i]
		}
	}
	if fallback != nil {
		log.WithFunc("snapshot.pickIndexChild").Warnf(ctx,
			"image-index has no linux/amd64 child, falling back to %s/%s (%s)",
			fallback.Platform.OS, fallback.Platform.Architecture, fallback.Digest)
		return *fallback, nil
	}
	return manifest.IndexManifest{}, errors.New("image-index has no usable platform child")
}

// writeImportTar is the original v1 assembly: every layer is one raw blob,
// streamed straight into its tar entry.
func writeImportTar(ctx context.Context, dl Downloader, name, localName string, cfg *manifest.SnapshotConfig, layers []manifest.Descriptor, w io.Writer, progress func(string)) error {
	bw := bufio.NewWriterSize(w, 256<<10)
	tw := tar.NewWriter(bw)

	now := nowFunc()
	if err := writeSnapshotEnvelope(tw, cfg, localName, now); err != nil {
		return err
	}

	for _, layer := range layers {
		title := layer.Title()
		if title == "" {
			return fmt.Errorf("layer %s missing %s annotation", layer.Digest, manifest.AnnotationTitle)
		}
		if progress != nil {
			progress(fmt.Sprintf("  %s (%d bytes)", title, layer.Size))
		}
		var fileMeta manifest.SnapshotFile
		if cfg.Files != nil {
			fileMeta = cfg.Files[title]
		}
		if err := streamLayerToTar(ctx, dl, name, layer, fileMeta, tw, now); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	return bw.Flush()
}

func writeSnapshotEnvelope(tw *tar.Writer, cfg *manifest.SnapshotConfig, localName string, now time.Time) error {
	envelope := snapshotExportEnvelope{
		Version: 1,
		Config: snapshotExportConfig{
			ID:           cfg.SnapshotID,
			Name:         localName,
			Description:  cfg.Description,
			Image:        cfg.Image,
			ImageDigest:  cfg.ImageDigest,
			ImageType:    cfg.ImageType,
			ImageBlobIDs: cfg.ImageBlobIDs,
			Hypervisor:   cfg.Hypervisor,
			CPU:          cfg.CPU,
			Memory:       cfg.Memory,
			Storage:      cfg.Storage,
			NICs:         cfg.NICs,
			Network:      cfg.Network,
			Windows:      cfg.Windows,
		},
	}
	envelopeJSON, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot envelope: %w", err)
	}
	envelopeJSON = append(envelopeJSON, '\n')

	if err := writeTarFile(tw, snapshotJSONName, envelopeJSON, 0o644, now); err != nil {
		return fmt.Errorf("write snapshot envelope: %w", err)
	}
	return nil
}

func layerHeader(title string, size int64, fileMeta manifest.SnapshotFile, modTime time.Time) (*tar.Header, error) {
	mode := fileMeta.Mode
	if mode == 0 {
		mode = 0o640
	}
	hdr := &tar.Header{
		Name:    title,
		Size:    size,
		Mode:    mode,
		ModTime: modTime,
	}
	if fileMeta.SparseMap != "" {
		if fileMeta.SparseSize <= 0 {
			return nil, fmt.Errorf("layer %s has sparse map without sparse size", title)
		}
		hdr.PAXRecords = map[string]string{
			sparsePAXMap:  fileMeta.SparseMap,
			sparsePAXSize: strconv.FormatInt(fileMeta.SparseSize, 10),
		}
	}
	return hdr, nil
}

func streamLayerToTar(ctx context.Context, dl Downloader, name string, layer manifest.Descriptor, fileMeta manifest.SnapshotFile, tw *tar.Writer, modTime time.Time) error {
	hdr, err := layerHeader(layer.Title(), layer.Size, fileMeta, modTime)
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}
	body, err := dl.GetBlob(ctx, name, layer.Digest)
	if err != nil {
		return fmt.Errorf("get blob %s: %w", layer.Digest, err)
	}
	defer func() { _ = body.Close() }()
	return ociutil.CopyBlobExact(tw, body, layer.Digest, layer.Size)
}

func writeTarFile(tw *tar.Writer, name string, data []byte, mode int64, modTime time.Time) error {
	if err := tw.WriteHeader(&tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    mode,
		ModTime: modTime,
	}); err != nil {
		return fmt.Errorf("write tar header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write tar body %s: %w", name, err)
	}
	return nil
}
