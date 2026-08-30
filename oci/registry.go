// Package oci provides a standard OCI Distribution registry client behind the
// snapshot Uploader/Downloader contracts, so artifacts can live in any registry.
package oci

import (
	"context"

	"github.com/cocoonstack/cocoon-common/snapshot"
)

// Registry is the OCI backend shared by vk-cocoon (push/pull) and the operator (probe/GC).
type Registry interface {
	snapshot.Uploader
	snapshot.Downloader
	HasManifest(ctx context.Context, repo, tag string) (bool, error)
	DeleteManifest(ctx context.Context, repo, reference string) error
}
