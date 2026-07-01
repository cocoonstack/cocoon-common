// Package oci provides a standard OCI Distribution registry client that
// satisfies the snapshot Uploader/Downloader contracts, so cocoon snapshots
// and cloud images can live in any OCI registry (e.g. Artifact Registry).
package oci

import (
	"context"

	"github.com/cocoonstack/cocoon-common/snapshot"
)

// Registry is the OCI backend shared by vk (push/pull) and the operator
// (existence probe + rollback).
type Registry interface {
	snapshot.Uploader
	snapshot.Downloader
	HasManifest(ctx context.Context, repo, tag string) (bool, error)
	DeleteManifest(ctx context.Context, repo, reference string) error
}
