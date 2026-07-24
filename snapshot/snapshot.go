// Package snapshot pushes and pulls cocoon VM snapshots as OCI artifacts.
// Push reads a `cocoon snapshot export` stream; Stream assembles a
// `cocoon snapshot import` tar into a caller-supplied writer.
package snapshot

import (
	"context"
	"errors"
	"io"
	"time"
)

const (
	snapshotJSONName = "snapshot.json"

	// cocoon uses custom PAX keys for sparse files; the stream preserves them.
	sparsePAXMap  = "COCOON.sparse.map"
	sparsePAXSize = "COCOON.sparse.size"

	// Default for PushOptions.Concurrency and StreamOptions.Concurrency:
	// parallel blob transfers (and, on push, aggregate encoder threads).
	defaultTransferConcurrency = 8
)

var (
	errMissingSnapshotJSON = errors.New("snapshot.json not found in export stream")

	nowFunc = time.Now // tests override
)

// Uploader abstracts OCI blob and manifest uploads.
type Uploader interface {
	HasBlob(ctx context.Context, name, digest string) (bool, error)
	PutBlob(ctx context.Context, name, digest string, body io.Reader, size int64) error
	PutManifest(ctx context.Context, name, tag string, data []byte, contentType string) error
}

// Downloader abstracts OCI manifest and blob downloads.
type Downloader interface {
	GetManifest(ctx context.Context, name, tag string) ([]byte, string, error)
	GetBlob(ctx context.Context, name, digest string) (io.ReadCloser, error)
}

// CocoonRunner abstracts the `cocoon snapshot export` source Pusher reads from.
type CocoonRunner interface {
	Export(ctx context.Context, name string) (io.ReadCloser, func() error, error)
}

type snapshotExportEnvelope struct {
	Version int                  `json:"version"`
	Config  snapshotExportConfig `json:"config"`
}

type snapshotExportConfig struct {
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Image        string              `json:"image,omitempty"`
	ImageDigest  string              `json:"image_digest,omitempty"`
	ImageType    string              `json:"image_type,omitempty"`
	ImageBlobIDs map[string]struct{} `json:"image_blob_ids,omitempty"`
	Hypervisor   string              `json:"hypervisor,omitempty"`
	CPU          int                 `json:"cpu,omitempty"`
	Memory       int64               `json:"memory,omitempty"`
	Storage      int64               `json:"storage,omitempty"`
	NICs         int                 `json:"nics,omitempty"`
	Network      string              `json:"network,omitempty"`
	Windows      bool                `json:"windows,omitempty"`
}
