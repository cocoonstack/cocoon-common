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
	"maps"
	"os"
	"strconv"
	"time"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/ociutil"
)

// PushOptions configures a snapshot push operation.
type PushOptions struct {
	Name        string
	Tag         string
	BaseImage   string // optional cocoonstack.snapshot.baseimage annotation
	Source      string
	Revision    string
	Annotations map[string]string
	Progress    func(string)

	// v2 wire-format knobs; all-zero reproduces the v1 writer exactly.
	ZstdLevel       int // >0: zstd-compress layers ≥ 1 MiB at this level
	ChunkSizeMiB    int // >0: split files into chunks of this many uncompressed MiB, one blob each
	Concurrency     int // parallel chunk uploads / encoder threads (default 8)
	MemoryBudgetMiB int // pipeline buffer cap (default 9216); workers clamp to budget/(2×chunk)−1
}

// Pusher exports and uploads cocoon snapshots as OCI artifacts.
type Pusher struct {
	Uploader Uploader
	Cocoon   CocoonRunner
}

// Push exports a snapshot via cocoon and uploads it as an OCI artifact.
func (p *Pusher) Push(ctx context.Context, opts PushOptions) error {
	if opts.Name == "" {
		return errors.New("snapshot push: name is required")
	}
	opts.Tag = cmp.Or(opts.Tag, "latest")

	stream, wait, err := p.Cocoon.Export(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("cocoon export %s: %w", opts.Name, err)
	}

	entries, readErr := p.readAndUploadEntries(ctx, opts, stream)
	// Close before wait so mid-tar failures unblock the subprocess.
	_ = stream.Close()
	waitErr := wait()
	if readErr != nil {
		return readErr
	}
	if waitErr != nil {
		return waitErr
	}
	if entries.cfg == nil {
		return errMissingSnapshotJSON
	}

	schemaVersion, artifactType := snapshotVersionMarkers(entries.encoded)
	configDescriptor, err := p.uploadSnapshotConfig(ctx, opts.Name, entries.cfg, entries.files, schemaVersion)
	if err != nil {
		return fmt.Errorf("upload snapshot config: %w", err)
	}

	manifestBytes, err := buildSnapshotManifest(configDescriptor, entries.layers, opts, artifactType)
	if err != nil {
		return fmt.Errorf("build manifest: %w", err)
	}

	if err := p.Uploader.PutManifest(ctx, opts.Name, opts.Tag, manifestBytes, manifest.MediaTypeOCIManifest); err != nil {
		return fmt.Errorf("put manifest %s:%s: %w", opts.Name, opts.Tag, err)
	}
	return nil
}

func (p *Pusher) uploadTarEntry(ctx context.Context, name string, hdr *tar.Header, body io.Reader) (manifest.Descriptor, error) {
	return p.uploadSpooled(ctx, name, hdr.Name, manifest.MediaTypeForCocoonFile(hdr.Name), func(w io.Writer) error {
		if _, err := io.Copy(w, io.LimitReader(body, hdr.Size)); err != nil {
			return fmt.Errorf("buffer entry: %w", err)
		}
		return nil
	})
}

// uploadSpooled spools a blob to a temp file while hashing, since the OCI API needs the digest before the first byte.
func (p *Pusher) uploadSpooled(ctx context.Context, name, title, mediaType string, write func(io.Writer) error) (manifest.Descriptor, error) {
	tmp, err := os.CreateTemp("", "cocoon-snapshot-*")
	if err != nil {
		return manifest.Descriptor{}, fmt.Errorf("create temp: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	h := sha256.New()
	if writeErr := write(io.MultiWriter(tmp, h)); writeErr != nil {
		return manifest.Descriptor{}, writeErr
	}
	written, err := tmp.Seek(0, io.SeekCurrent)
	if err != nil {
		return manifest.Descriptor{}, fmt.Errorf("size temp: %w", err)
	}

	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if err := p.putBlobIfMissing(ctx, name, digest, tmp, written); err != nil {
		return manifest.Descriptor{}, err
	}
	return manifest.Descriptor{
		MediaType:   mediaType,
		Digest:      digest,
		Size:        written,
		Annotations: map[string]string{manifest.AnnotationTitle: title},
	}, nil
}

func (p *Pusher) putBlobIfMissing(ctx context.Context, name, digest string, body io.ReadSeeker, size int64) error {
	exists, err := p.Uploader.HasBlob(ctx, name, digest)
	if err != nil {
		return fmt.Errorf("check blob %s: %w", digest, err)
	}
	if exists {
		return nil
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek blob %s: %w", digest, err)
	}
	if err := p.Uploader.PutBlob(ctx, name, digest, body, size); err != nil {
		return fmt.Errorf("put blob %s: %w", digest, err)
	}
	return nil
}

func (p *Pusher) uploadSnapshotConfig(ctx context.Context, name string, cfg *snapshotExportConfig, files map[string]manifest.SnapshotFile, schemaVersion string) (manifest.Descriptor, error) {
	cfgBlob := manifest.SnapshotConfig{
		SchemaVersion: schemaVersion,
		SnapshotID:    cfg.ID,
		Description:   cfg.Description,
		Image:         cfg.Image,
		ImageDigest:   cfg.ImageDigest,
		ImageType:     cfg.ImageType,
		ImageBlobIDs:  cfg.ImageBlobIDs,
		Hypervisor:    cfg.Hypervisor,
		CPU:           cfg.CPU,
		Memory:        cfg.Memory,
		Storage:       cfg.Storage,
		NICs:          cfg.NICs,
		Network:       cfg.Network,
		Windows:       cfg.Windows,
		Files:         files,
		CreatedAt:     nowFunc().UTC(),
	}
	data, err := json.Marshal(cfgBlob)
	if err != nil {
		return manifest.Descriptor{}, fmt.Errorf("marshal snapshot config: %w", err)
	}

	digest := "sha256:" + ociutil.SHA256Hex(data)
	if err := p.putBlobIfMissing(ctx, name, digest, bytes.NewReader(data), int64(len(data))); err != nil {
		return manifest.Descriptor{}, err
	}
	return manifest.Descriptor{
		MediaType: manifest.MediaTypeSnapshotConfig,
		Digest:    digest,
		Size:      int64(len(data)),
	}, nil
}

func buildSnapshotManifest(config manifest.Descriptor, layers []manifest.Descriptor, opts PushOptions, artifactType string) ([]byte, error) {
	annotations := map[string]string{
		manifest.AnnotationCreated: nowFunc().UTC().Format(time.RFC3339),
	}
	if opts.BaseImage != "" {
		annotations[manifest.AnnotationSnapshotBaseImage] = opts.BaseImage
	}
	if opts.Source != "" {
		annotations[manifest.AnnotationSource] = opts.Source
	}
	if opts.Revision != "" {
		annotations[manifest.AnnotationRevision] = opts.Revision
	}
	maps.Copy(annotations, opts.Annotations)

	m := manifest.OCIManifest{
		SchemaVersion: 2,
		MediaType:     manifest.MediaTypeOCIManifest,
		ArtifactType:  artifactType,
		Config:        config,
		Layers:        layers,
		Annotations:   annotations,
	}
	return json.MarshalIndent(m, "", "  ")
}

// snapshotVersionMarkers pairs the config schemaVersion with the manifest artifactType; the reader trusts them to agree.
func snapshotVersionMarkers(encoded bool) (schemaVersion, artifactType string) {
	if encoded {
		return "v2", manifest.ArtifactTypeSnapshotV2
	}
	return "v1", manifest.ArtifactTypeSnapshot
}

func sparseFileMeta(hdr *tar.Header) (manifest.SnapshotFile, error) {
	fileMeta := manifest.SnapshotFile{Mode: hdr.Mode}
	sparseMap, ok := hdr.PAXRecords[sparsePAXMap]
	if !ok {
		return fileMeta, nil
	}
	fileMeta.SparseMap = sparseMap
	rawSize, ok := hdr.PAXRecords[sparsePAXSize]
	if !ok {
		return fileMeta, fmt.Errorf("sparse entry %s missing %s", hdr.Name, sparsePAXSize)
	}
	sparseSize, parseErr := strconv.ParseInt(rawSize, 10, 64)
	if parseErr != nil {
		return fileMeta, fmt.Errorf("parse sparse size for %s: %w", hdr.Name, parseErr)
	}
	fileMeta.SparseSize = sparseSize
	return fileMeta, nil
}
