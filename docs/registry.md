# Registry and snapshots

Cocoon VM snapshots and cloud images travel between nodes as OCI artifacts.
Five packages cover that path, and every component that touches a registry
shares them so producer and consumer cannot drift on the wire format.

```
oci       Registry interface + standard-OCI implementation
snapshot  Pusher / Stream — the snapshot wire format
cloudimg  Stream — cloud-image disk artifacts
manifest  OCI manifest types, media types, classification
ociutil   reference parsing, digest/size-verified blob copies
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

Two transport decisions are load-bearing:

- The client pins one puller and one pusher via `remote.Reuse`, so the `/v2/`
  ping and bearer-token exchange happen once per repo instead of once per blob
  call.
- HTTP/2 is disabled and idle connections per host are raised. A bulk transfer
  multiplexed onto a single HTTP/2 connection is head-of-line blocked; several
  HTTP/1.1 connections saturate the link.

`DeleteManifest` treats a registry 404 as success — every caller wants
ensure-absent, and a GC path would otherwise log errors for tags that were
never pushed. `GetManifest` maps a 404 to `snapshot.ErrManifestNotFound`, the
typed "tag is absent" every caller distinguishes from a transport failure: an
absent hibernate tag is a legitimate state, a 500 is not.

## Snapshot push

```go
p := &snapshot.Pusher{Uploader: reg, Cocoon: runner}
res, err := p.Push(ctx, snapshot.PushOptions{
    Name:      "myvm",
    Tag:       meta.DefaultSnapshotTag,
    BaseImage: img,     // guards a wake against an image swap
})
```

`Push` reads a `cocoon snapshot export` tar through the `CocoonRunner`
interface, uploads one blob per file, and publishes an OCI manifest whose
config blob is a `manifest.SnapshotConfig`. `PushOptions.Annotations` adds
caller annotations to that manifest; `Source` and `Revision` map to the
standard `org.opencontainers.image.*` keys.

Layer blobs are content-addressed and preflighted with `HasBlob`, so a second
push of an unchanged VM re-uploads no layers. The config blob and the manifest
always go over the wire: the config embeds a fresh `CreatedAt`, so its digest
differs on every push.

## Wire formats: v1 and v2

| | v1 | v2 |
|---|---|---|
| artifactType | `…snapshot.v1+json` | `…snapshot.v2+json` |
| config `schemaVersion` | `v1` | `v2` |
| layers | one blob per file, raw | optionally zstd-compressed and/or split into fixed-size chunks |
| chunk order | n/a | `SnapshotConfig.Files[].Chunks`, an ordered digest list |

The four v2 knobs are all opt-in:

| Option | Effect |
|---|---|
| `ZstdLevel` | `>0` compresses layers ≥ 1 MiB at that level |
| `ChunkSizeMiB` | `>0` splits files into chunks of that many uncompressed MiB, one blob each |
| `Concurrency` | parallel chunk uploads and encoder threads (default 8) |
| `MemoryBudgetMiB` | pipeline buffer cap (default 9216) |

An all-zero `PushOptions` reproduces the v1 writer exactly, so an
unconfigured pusher stays readable by a v1-only puller. Turning the knobs on
does not by itself produce a v2 artifact: if nothing in the export is large
enough to compress or split, the manifest is still classified v1.

Both buffer pools hold `workers+1` chunks, so the effective worker count
solves `2 × (workers+1) × chunkSize ≤ budget`. A chunk size whose single-worker
floor (`4 × chunkSize`) exceeds the budget is rejected up front rather than
silently degraded.

## Snapshot pull

```go
err := snapshot.Stream(ctx, rawManifest, reg, snapshot.StreamOptions{
    Name:      "myvm",
    LocalName: "restored",   // empty = Name
    Writer:    w,
})
```

`Stream` accepts raw manifest bytes and resolves an OCI image-index to a child
manifest (preferring `linux/amd64`, falling back to the first non-attestation
entry) before assembling; `StreamParsed` takes an already-parsed manifest. The
output is a `cocoon snapshot import` tar written to any `io.Writer`.

Validation fails closed before the first byte is streamed: every layer must
carry a decodable media type and a title annotation, and compressed or chunked
layers may appear only in a v2 manifest. An unknown media type is an error,
not a passthrough — a newer writer must not be silently mis-assembled by an
older reader.

Chunked files are prefetched in parallel under an explicit memory budget
(`Concurrency`, `MemoryBudgetMiB`, default 4 GiB). When the budget cannot hold
two chunk buffers, or a chunk is larger than 1 GiB, the same file streams
sequentially instead — the output is byte-identical either way.

`snapshot.FetchSnapshotConfig` fetches just the config blob, which is enough to
decide whether a local copy still matches the tag before committing to a
transfer. `snapshot.MarshalEnvelope` re-emits that config as the
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

`ociutil` is where digest and size enforcement lives, so no consumer
hand-rolls a hash check:

- `CopyBlobSized` — exact-size and no-trailing-data enforcement for a body the
  transport already digest-verified
- `CopyBlobExact` — the same, plus a full sha256 re-hash, for transports that
  do not verify
- `ParseRef` / `IsRelativeRef` — registry-relative `repo[:tag]` parsing, with
  the guard that keeps a host:port or a digest from being split as a tag
