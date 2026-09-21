# Registry and snapshots

Cocoon VM snapshots and cloud images travel between nodes as OCI artifacts.
Five packages define that path. Consumers share the wire types and transfer
code through the cocoon-common version pinned in their own modules.

```
oci       Registry interface + standard-OCI implementation
snapshot  Pusher / Stream — the snapshot wire format
cloudimg  Stream — cloud-image disk artifacts
manifest  OCI manifest types, media types, classification
ociutil   reference parsing and size checks for digest-verified blob streams
```

## The Registry interface

`oci.Registry` is what consumers depend on:

```go
type Registry interface {
    snapshot.Uploader    // HasBlob, PutBlob, PutManifest
    snapshot.Downloader  // GetManifest, GetBlob
    HasManifest(ctx context.Context, repo, tag string) (bool, error)
    DeleteManifest(ctx context.Context, repo, reference string) error
}
```

That split is exactly the split between vk-cocoon (push / pull) and
cocoon-operator (existence probe, tag GC).
`oci.NewOCIRegistry(base, keychain)` is the standard-OCI implementation, built
on `go-containerregistry` with keychain auth; a test or an alternative backend
substitutes the interface.

The standard transport uses two optimizations:

- The client reuses a puller and a pusher via `remote.Reuse` so repository
  authentication setup can be shared across blob calls.
- HTTP/2 is disabled and up to 32 idle connections are retained per host,
  allowing parallel transfers over separate HTTP/1.1 connections.

`DeleteManifest` treats a registry 404 as success — every caller wants
ensure-absent, and a GC path would otherwise log errors for tags that were
never pushed. `GetManifest` maps a 404 to `snapshot.ErrManifestNotFound`, the
typed "tag is absent" every caller distinguishes from a transport failure: an
absent hibernate tag is a legitimate state, a 500 is not.

## Snapshot push

```go
p := &snapshot.Pusher{Uploader: reg, Cocoon: runner}
err := p.Push(ctx, snapshot.PushOptions{
    Name:      "myvm",
    Tag:       meta.DefaultSnapshotTag,
    BaseImage: img,     // guards a wake against an image swap
})
```

`Push` reads a `cocoon snapshot export` tar through the `CocoonRunner`
interface, uploads file data as raw or encoded layers, and publishes an OCI
manifest whose config blob is a `manifest.SnapshotConfig`. `PushOptions.Annotations` adds
caller annotations to that manifest; `Source` and `Revision` map to the
standard `org.opencontainers.image.*` keys. Caller annotations are applied last
and can override the generated annotation values.

The config also preserves the complete `snapshot.json` config object in
`SnapshotConfig.Engine`, including fields this library does not model.
`MarshalEnvelope` restores that object with only `name` replaced by the local
name, preserving JSON number precision. Configs without `Engine` use the
typed legacy fields. This preservation applies to both v1 and v2 layers.

Layer blobs are content-addressed and preflighted with `HasBlob`, so a second
push of unchanged file data with the same encoding settings re-uploads no
layers. The config blob also uses `HasBlob`, but a fresh `CreatedAt` normally
gives each push a new config digest. The manifest is published after all
layer and config uploads succeed.

## Wire formats: v1 and v2

| | v1 | v2 |
|---|---|---|
| artifactType | `…snapshot.v1+json` | `…snapshot.v2+json` |
| config `schemaVersion` | `v1` | `v2` |
| layers | one blob per file, raw | optionally zstd-compressed and/or split into fixed-size chunks |
| chunk order | n/a | `SnapshotConfig.Files[].Chunks`, an ordered digest list |

Compression and chunking are opt-in; concurrency and memory budget tune the
transfer:

| Option | Effect |
|---|---|
| `ZstdLevel` | `>0` compresses layers ≥ 1 MiB at that level |
| `ChunkSizeMiB` | `>0` splits files into chunks of that many uncompressed MiB, one blob each |
| `Concurrency` | parallel chunk uploads and encoder threads (default 8) |
| `MemoryBudgetMiB` | pipeline buffer cap (default 9216) |

Leaving these four options zero produces a v1-compatible artifact (the config
additionally carries `files[].size`), so an unconfigured pusher stays
readable by a v1-only puller. Turning the knobs on
does not by itself produce a v2 artifact: if nothing in the export is large
enough to compress or split, the manifest is still classified v1.

Both buffer pools hold `workers+1` chunks, so the effective worker count
solves `2 × (workers+1) × chunkSize ≤ budget`. A chunk size whose single-worker
floor (`4 × chunkSize`) exceeds the budget is rejected up front rather than
silently degraded. `ChunkSizeMiB` is capped at 4096. Without chunking,
compressed files are spooled to temporary files and uploaded sequentially.

## Snapshot pull

```go
err := snapshot.Stream(ctx, rawManifest, reg, snapshot.StreamOptions{
    Name:      "myvm",
    LocalName: "restored",   // empty = Name
    Writer:    w,
})
```

`Stream` accepts raw manifest bytes and resolves an OCI image-index to a child
manifest (preferring `linux/amd64`, then the first entry with a non-nil platform
whose architecture is not `unknown`) before assembling; `StreamParsed` takes
an already-parsed manifest. The output is a `cocoon snapshot import` tar
written to any `io.Writer`.

Validation fails closed before the first byte is streamed: every layer must
carry a decodable media type and a title annotation, and compressed or chunked
layers may appear only in a v2 manifest. An unknown media type is an error,
not a passthrough — a newer writer must not be silently mis-assembled by an
older reader.

Chunked files are prefetched in parallel under an explicit memory budget
(`Concurrency` defaults to 8; `MemoryBudgetMiB` defaults to 4096). Prefetch needs
at least two workers, their input/output buffers, and one extra output buffer.
It falls back to sequential streaming when that does not fit, concurrency is
below two, or a computed buffer capacity exceeds 1 GiB. The output is the same
in either mode. Chunks are uniform: every chunk but the last holds
`ChunkSizeMiB` MiB of uncompressed data. The pull side sizes its decode buffers
from that invariant (`rawChunkStride`), so a
producer that chunked unevenly would have to record the largest raw chunk in
the config first and update the reader to use it.

If a prefetched read or output write fails, the pull cancels outstanding
chunk reads and waits for their buffers to be returned before closing the
shared decoder. Custom `Downloader.GetBlob` bodies must observe context
cancellation for that cleanup to finish.

`snapshot.FetchSnapshotConfig` fetches just the config blob, capped at 64 MiB,
so a caller can compare it with a local snapshot before transferring layers.
`snapshot.MarshalEnvelope` re-emits that config as the
`snapshot.json` cocoon expects beside exported files, so bytes staged from a
peer keep the registry as their identity anchor.

## Cloud images

`cloudimg.Stream(ctx, raw, blobs, w)` classifies the manifest, then
concatenates its disk layers — sorted by title annotation, since large images
are split across layers to stay under registry per-layer limits — onto `w`.
The layers are copied verbatim, so the stream is qcow2 or raw depending on the
artifact's own disk media types.

## Media types and classification

`manifest` holds the OCI manifest and descriptor types, the cocoon
`SnapshotConfig`, and the vocabulary:

- `Classify(raw)` / `ClassifyParsed(m)` → `KindContainerImage`,
  `KindCloudImage`, `KindSnapshot`, `KindImageIndex`, `KindUnknown`
- `MediaTypeForCocoonFile(name)` maps an export-tar filename to its layer
  media type
- `ZstdMediaType` / `IsZstdMediaType` / `StripZstd` handle the `+zstd` suffix
- `IsSnapshotLayerMediaType` is the reader's allowlist
- `IsDiskMediaType` covers the disk layers of a cloud image

## Blob verification

`Downloader.GetBlob` returns a digest-verified stream; `ociutil` enforces the
descriptor size without hashing the body a second time:

- `CopyBlobSized` — exact-size and no-trailing-data enforcement for a body the
  transport already digest-verified
- `ParseRef` splits registry-relative `repo[:tag]` strings at the first colon,
  defaulting a missing tag to `latest`; it does not validate the input
- `IsRelativeRef` validates that grammar before `ParseRef` is used on external
  input, rejecting URLs, registry ports, digests, and empty tags
