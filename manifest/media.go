package manifest

import "strings"

const (
	MediaTypeOCIManifest = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeOCIIndex    = "application/vnd.oci.image.index.v1+json"
	MediaTypeDockerIndex = "application/vnd.docker.distribution.manifest.list.v2+json"

	MediaTypeOCIImageConfig = "application/vnd.oci.image.config.v1+json"
	MediaTypeDockerConfig   = "application/vnd.docker.container.image.v1+json"

	ArtifactTypeOSImage = "application/vnd.cocoonstack.os-image.v1+json"
	// ArtifactTypeWindowsImage is the legacy name; both are recognized.
	ArtifactTypeWindowsImage = "application/vnd.cocoonstack.windows-image.v1+json"
	ArtifactTypeSnapshot     = "application/vnd.cocoonstack.snapshot.v1+json"
	// SnapshotV2 marks snapshots with compressed and/or chunked layers; readers
	// that predate it classify the manifest as unknown and fail before touching layers.
	ArtifactTypeSnapshotV2 = "application/vnd.cocoonstack.snapshot.v2+json"

	MediaTypeSnapshotConfig = "application/vnd.cocoonstack.snapshot.config.v1+json"

	// *Part variants are for split disks (GHCR per-layer size limit).
	MediaTypeDiskQcow2     = "application/vnd.cocoonstack.disk.qcow2"
	MediaTypeDiskQcow2Part = "application/vnd.cocoonstack.disk.qcow2.part"
	MediaTypeDiskRaw       = "application/vnd.cocoonstack.disk.raw"
	MediaTypeDiskRawPart   = "application/vnd.cocoonstack.disk.raw.part"

	MediaTypeWindowsDiskQcow2     = "application/vnd.cocoonstack.windows.disk.qcow2"
	MediaTypeWindowsDiskQcow2Part = "application/vnd.cocoonstack.windows.disk.qcow2.part"
	MediaTypeWindowsDiskRaw       = "application/vnd.cocoonstack.windows.disk.raw"
	MediaTypeWindowsDiskRawPart   = "application/vnd.cocoonstack.windows.disk.raw.part"

	MediaTypeVMConfig = "application/vnd.cocoonstack.vm.config+json"
	MediaTypeVMState  = "application/vnd.cocoonstack.vm.state+json"
	MediaTypeVMMemory = "application/vnd.cocoonstack.vm.memory"
	MediaTypeVMCidata = "application/vnd.cocoonstack.vm.cidata"

	MediaTypeGeneric = "application/octet-stream"
	MediaTypeTar     = "application/x-tar"

	AnnotationTitle    = "org.opencontainers.image.title"
	AnnotationCreated  = "org.opencontainers.image.created"
	AnnotationSource   = "org.opencontainers.image.source"
	AnnotationRevision = "org.opencontainers.image.revision"

	AnnotationSnapshotBaseImage = "cocoonstack.snapshot.baseimage"

	// Chunked-layer annotations are debugging aids; SnapshotConfig.Files[].Chunks
	// is the authoritative chunk order.
	AnnotationChunkIndex = "cocoonstack.chunk.index"
	AnnotationChunkCount = "cocoonstack.chunk.count"

	zstdSuffix = "+zstd"
)

// ZstdMediaType returns the mediaType for the zstd-compressed variant of mt.
func ZstdMediaType(mt string) string {
	return mt + zstdSuffix
}

// IsZstdMediaType reports whether mt denotes a zstd-compressed layer.
func IsZstdMediaType(mt string) bool {
	return strings.HasSuffix(mt, zstdSuffix)
}

// StripZstd returns mt without a trailing +zstd suffix, if present.
func StripZstd(mt string) string {
	return strings.TrimSuffix(mt, zstdSuffix)
}

// IsSnapshotLayerMediaType reports whether mt is a snapshot layer mediaType the
// reader knows how to decode (a known raw type or its +zstd variant). Unknown
// types must fail the pull rather than reach `cocoon snapshot import`.
func IsSnapshotLayerMediaType(mt string) bool {
	switch StripZstd(mt) {
	case MediaTypeVMConfig, MediaTypeVMState, MediaTypeVMMemory, MediaTypeVMCidata,
		MediaTypeDiskQcow2, MediaTypeDiskRaw, MediaTypeGeneric:
		return true
	}
	return false
}

var snapshotFilenameMediaType = map[string]string{
	"config.json":   MediaTypeVMConfig,
	"state.json":    MediaTypeVMState,
	"memory-ranges": MediaTypeVMMemory,
	"cidata.img":    MediaTypeVMCidata,
	"overlay.qcow2": MediaTypeDiskQcow2,
}

// MediaTypeForCocoonFile returns the layer mediaType for a cocoon snapshot tar file.
func MediaTypeForCocoonFile(name string) string {
	if mt, ok := snapshotFilenameMediaType[name]; ok {
		return mt
	}
	switch {
	case strings.HasSuffix(name, ".qcow2"):
		return MediaTypeDiskQcow2
	case strings.HasSuffix(name, ".raw"):
		return MediaTypeDiskRaw
	}
	return MediaTypeGeneric
}

// IsDiskMediaType reports whether mt is a disk layer mediaType.
func IsDiskMediaType(mt string) bool {
	switch mt {
	case MediaTypeDiskQcow2, MediaTypeDiskQcow2Part,
		MediaTypeDiskRaw, MediaTypeDiskRawPart,
		MediaTypeWindowsDiskQcow2, MediaTypeWindowsDiskQcow2Part,
		MediaTypeWindowsDiskRaw, MediaTypeWindowsDiskRawPart:
		return true
	}
	return false
}
