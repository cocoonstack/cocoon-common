# cocoon-common

Shared Go packages for [cocoonstack](https://github.com/cocoonstack) services.

## Overview

- `apis/v1` -- typed CocoonSet and CocoonHibernation CRD Go types and generated CRD YAML manifests
- `meta` -- shared CRD identifiers, annotation/label/toleration keys, VM naming helpers, the typed `VMSpec` / `VMRuntime` / `HibernateState` / `LifecycleStatus` annotation contract, and pod-state helpers (`IsPodReady`, `IsPodTerminal`, `IsContainerRunning`, `PodKey`, `PodNodePool`) every cocoon component shares
- `k8s` -- Kubernetes client config bootstrap with the standard kubeconfig fallback chain, merge-patch helpers, env/sleep helpers (`EnvOrDefault`, `EnvBool`, `SleepCtx`), unstructured decoder, and TLS helpers (`LoadOrGenerateCert`, `GenerateSelfSignedCert`, `DetectNodeIP`)
- `k8s/admission` -- shared admission-webhook scaffolding (`Allow` / `Deny` responses, `Decode` / `Serve` request loop) used by `cocoon-webhook` and reusable by any future cocoonstack admission handler
- `snapshot` -- push cocoon VM snapshots to an OCI registry and stream them back, including the chunked/zstd v2 wire format
- `cloudimg` -- stream a cocoonstack cloud-image (qcow2) artifact out of a registry into `cocoon image import`
- `oci` -- the `Registry` interface every consumer codes against, plus the standard-OCI implementation behind it
- `manifest` -- OCI manifest / descriptor types, the cocoon snapshot config, and media-type classification
- `ociutil` -- reference parsing and digest/size-verified blob copies shared by the registry paths
- `httpx` -- HTTP server bootstrap and coordinated graceful shutdown for cocoonstack binaries
- `log` -- common log setup for cocoonstack binaries using `projecteru2/core/log`

This repository keeps cross-project contracts in one place instead of re-exporting them from `cocoon-operator`. `cocoon-operator`, `cocoon-webhook`, `vk-cocoon`, and `cocoon-net` all consume the same package set directly.

## Installation

### Add dependency

```bash
go get github.com/cocoonstack/cocoon-common@latest
```

### Build from source

```bash
git clone https://github.com/cocoonstack/cocoon-common.git
cd cocoon-common
make build
```

## Packages

### `apis/v1`

Typed Go definitions for the `cocoonset.cocoonstack.io/v1` API group, plus the generated CRD YAML manifests under `apis/v1/crds/`. The package ships:

- `CocoonSet` -- declarative spec for an agent cluster (main + sub-agents + toolboxes)
- `CocoonHibernation` -- per-pod hibernate / wake request

Two CEL rules ship inside the generated CocoonSet CRD, so they hold even for a client that never touches these Go types: `hibernatePolicy: release` requires a main-only set (`agent.replicas=0`, no toolboxes), and `snapshotCompatibilityClass` is immutable once set — a snapshot class change would invalidate every memory snapshot the set has already published.

Downstream operators import these via `go list -m` and copy the CRD YAML into their own kustomize tree (see `make import-crds` in cocoon-operator). Regenerate via `make generate manifests` after any type change.

### `meta`

Use `meta` for:

- Cocoon annotation, label, and CRD identifier constants
- stable VM naming helpers
- slot extraction and role inference
- toolbox connection type detection

#### Identifier namespaces

All identifiers live under three cocoonstack.io prefixes:

| Prefix | Used for | Examples |
|---|---|---|
| `cocoonset.cocoonstack.io/` | CocoonSet CRD group, Pod selector labels, and CocoonSet-level fields the operator mirrors onto a managed Pod | `cocoonset.cocoonstack.io/v1`, `name`, `role`, `slot`, `mode`, `image`, `os`, `storage`, `snapshot-policy`, `network`, `managed`, `force-pull`, `generation`, `hibernated-on-node` |
| `vm.cocoonstack.io/` | VM-instance metadata — observed runtime state plus per-VM spec the operator hands to vk-cocoon | `id`, `name`, `ip`, `vnc-port`, `hibernate`, `restore-from-hibernate`, `keep-snapshot-on-delete`, `fork-from`, `clone-from-dir`, `conn-type`, `backend`, `no-direct-io`, `probe-port`, `lifecycle-state`, `lifecycle-observed-generation`, `lifecycle-state-message` |
| `cocoonstack.io/` | Node labels vk-cocoon stamps on its virtual node and the operator selects on | `pool`, `snapshot-cpu-class` |

For typed annotation access, prefer the `meta.VMSpec` / `meta.VMRuntime` / `meta.HibernateState` wrappers over raw map manipulation:

```go
// Managed=true: vk-cocoon owns lifecycle; false: adopt pre-assigned VM.
spec := meta.VMSpec{
    VMName:         "vk-prod-demo-0",
    Image:          "ghcr.io/cocoonstack/cocoon/ubuntu:24.04",
    Mode:           string(v1.AgentModeRun),
    OS:             string(v1.OSLinux),
    Backend:        string(v1.BackendFirecracker),
    SnapshotPolicy: string(v1.SnapshotPolicyAlways),
    Managed:        true,
    ForcePull:      true,  // bypass image cache
    ProbePort:      "22",  // TCP readiness probe on port 22
}
spec.Apply(pod)

// vk-cocoon side: write runtime state back to the pod.
runtime := meta.VMRuntime{VMID: vmID, IP: ip}
runtime.Apply(pod)

// hibernate / wake
meta.HibernateState(true).Apply(pod)
```

Two snapshot tag constants anchor the cross-component contract:

- `meta.HibernateSnapshotTag` (`"hibernate"`) — the OCI tag vk-cocoon pushes a hibernation snapshot under, and the tag the operator probes to detect that a hibernation has completed.
- `meta.DefaultSnapshotTag` (`"latest"`) — the tag vk-cocoon publishes routine (non-hibernate) VM snapshots under at pod-delete time, and the tag cocoon-operator garbage-collects when a CocoonSet is deleted.

`meta.ShouldSnapshotVM(spec, role)` is the single shared decoder for the `SnapshotPolicy` / role decision. vk-cocoon consults it on the producer side (should I push this VM?) and cocoon-operator on the GC side (should I delete this tag?) so the two cannot drift — under `main-only` both sides agree only the main agent is touched. The role comes from the pod's CocoonSet ownership (`meta.RoleForPod`), never from a VM-name suffix.

`meta.LabelSnapshotCompatibilityClass` (`cocoonstack.io/snapshot-cpu-class`) closes the same loop for placement: `CocoonSetSpec.SnapshotCompatibilityClass` names a certified guest-visible CPU ABI, cocoon-operator renders it as a hard node selector on every managed pod, and vk-cocoon publishes the label on nodes configured with that class and refuses any classified pod it cannot serve. It is independent from `LabelNodePool`, so several workload pools can share one snapshot class.

`meta.AnnotationKeepSnapshotOnDelete` marks a pod delete as a scheduling-seat release rather than a teardown: vk-cocoon keeps the node-local snapshot as the warm-wake cache instead of dropping it with the VM. It is best-effort by contract — a lost flag costs the wake a registry pull, never correctness.

`meta.LifecycleStatus` is the typed contract for the lifecycle-state annotation triple vk-cocoon writes (`state`, `observed-generation`, `message`):

```go
status := meta.LifecycleStatus{
    State:              meta.LifecycleStateReady,
    ObservedGeneration: meta.ReadCocoonSetGeneration(pod),
}

// In-memory: mutate the pod we already hold.
status.Apply(pod)

// Wire: status.Annotations() returns the annotation key/value map
// (nil values delete the key); wrap with k8s.AnnotationsMergePatch
// for an apiserver merge-patch body.
patch, _ := k8s.AnnotationsMergePatch(status.Annotations())
```

cocoon-operator stamps the owning CocoonSet's `metadata.generation` onto the pod via `meta.StampCocoonSetGeneration` so vk-cocoon can echo it back as `lifecycle-observed-generation`. Counter-based completion lets clients tell "the operation I asked for finished" from "an older completion is still being reported", without depending on wall-clock skew.

### `k8s`

Use `k8s.LoadConfig()` to resolve cluster configuration from:

1. `KUBECONFIG`
2. `~/.kube/config`
3. in-cluster config

Other helpers in this package:

- `k8s.NewClientset` / `k8s.NewClientsetAndDynamic` -- clientset constructors on top of `LoadConfig`, for binaries that do not run a controller-runtime manager.
- `k8s.EnvOrDefault`, `k8s.EnvBool` -- lenient env-var parsing that falls back to the supplied default on unset / malformed values.
- `k8s.SleepCtx(ctx, d)` -- context-aware sleep; returns `false` when the context fires first so callers can exit retry loops without a second `select`.
- `k8s.RunTicker(ctx, interval, fn)` -- the repeat-until-canceled counterpart to `SleepCtx`, so background reconcile loops stop open-coding a ticker plus `select`.
- `k8s.LoadOrGenerateCert` / `k8s.GenerateSelfSignedCert` / `k8s.DetectNodeIP` -- TLS bring-up helpers used by `vk-cocoon` and reusable by any cocoonstack HTTP server that needs a dev-time self-signed fallback. `DetectNodeIP` returns `(string, error)`.
- `k8s.StatusMergePatch` / `k8s.AnnotationsMergePatch` -- merge-patch builders used by reconcilers that prefer the JSON merge-patch encoding over `client.MergeFrom`.
- `k8s.PatchStatus[T]` -- generic `client.MergeFrom` patch for the `/status` subresource; captures the pre-mutation snapshot via the kubebuilder-generated typed `DeepCopy()` so callers skip the boilerplate.
- `k8s.PatchHibernateState` -- pod-level hibernate annotation patch that short-circuits when the pod already carries the desired state, safe to call unconditionally in a reconcile loop.
- `k8s.PatchCocoonSetGeneration` -- stamps the owning CocoonSet's `metadata.generation` onto the pod for vk-cocoon to echo back as `lifecycle-observed-generation`; same short-circuit semantics as `PatchHibernateState`.
- `k8s.PatchKeepSnapshotOnDelete` -- flags a pod so vk-cocoon keeps its node-local snapshot when the delete is a seat release.
- `k8s.Patch[T]` -- the same `client.MergeFrom` mutate-and-patch shape as `PatchStatus[T]`, for the main resource instead of `/status`.
- `k8s.NewReadyCondition` / `k8s.ConditionTypeReady` -- canonical `Ready` condition constructor shared across every cocoon CRD status block, leaving `LastTransitionTime` zero so `apimeta.SetStatusCondition` preserves the existing timestamp on no-op updates.
- `k8s.DecodeUnstructured[T]` -- generic unstructured-to-typed converter.

### `k8s/admission`

Shared admission-webhook scaffolding. Example:

```go
import commonadmission "github.com/cocoonstack/cocoon-common/k8s/admission"

mux.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
    commonadmission.Serve(w, r, 0 /* default max body */, func(ctx context.Context, rev *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
        // ... your handler logic ...
        return commonadmission.Allow()
    })
})
```

### `snapshot`, `cloudimg`, `oci`, `manifest`, `ociutil`

The registry side of the platform: cocoon VM snapshots and cloud images travel between nodes as OCI artifacts, and every component that touches them shares this code so producer and consumer cannot drift on the wire format.

- `oci.Registry` is the interface consumers depend on — `snapshot.Uploader` + `snapshot.Downloader` plus `HasManifest` / `DeleteManifest`, which is exactly the split between vk-cocoon (push / pull) and cocoon-operator (existence probe, tag GC). `oci.NewOCIRegistry(base, keychain)` is the standard-OCI implementation; a test or an alternative backend substitutes the interface.
- `snapshot.Pusher.Push` exports a snapshot through the `CocoonRunner` and uploads it. `PushOptions` carries the tag, the optional `cocoonstack.snapshot.baseimage` annotation that guards a wake against an image swap, caller annotations, and the v2 wire-format knobs (`ZstdLevel`, `ChunkSizeMiB`, `Concurrency`, `MemoryBudgetMiB`) — all-zero reproduces the v1 writer exactly, so an unconfigured pusher stays readable by a v1-only puller.
- `snapshot.Stream` / `StreamParsed` assemble a snapshot tar back out of the registry, prefetching chunks in parallel under an explicit memory budget; `snapshot.FetchSnapshotConfig` fetches just the config blob — enough to decide whether a local copy still matches the tag before committing to a transfer — and `snapshot.MarshalEnvelope` re-emits that config as the `snapshot.json` cocoon expects beside exported files, so bytes staged from a peer keep the registry as their identity anchor. `snapshot.ErrManifestNotFound` is the typed "tag is absent" every caller distinguishes from a transport failure — an absent hibernate tag is a legitimate state, a 500 is not.
- `cloudimg.Stream` does the same for a cocoonstack cloud-image artifact, writing the qcow2 to any `io.Writer`.
- `manifest` holds the OCI manifest / descriptor types, the cocoon `SnapshotConfig`, and the media-type predicates (`Classify`, `IsSnapshotLayerMediaType`, `IsDiskMediaType`, `StripZstd`) that decide what an artifact actually is.
- `ociutil` covers reference parsing (`ParseRef`, `IsRelativeRef`) and digest/size-verified blob copies, so no consumer hand-rolls a hash check.

### `httpx`

`httpx.Run(ctx, shutdownTimeout, specs...)` starts one or more servers built by `httpx.NewServer` (which applies `DefaultReadHeaderTimeout`) and shuts them all down together when the context fires or any one of them fails. Both plain and TLS servers register through `HTTPServerSpec` / `HTTPSServerSpec`.

### `log`

Use `log.Setup(ctx, envVar) error` to initialize the shared logger from an environment variable, defaulting to `info`. Returns an error if the level value is invalid.

## Development

```bash
make build          # build all packages
make test           # run tests with coverage
make lint           # run golangci-lint on linux + darwin
make fmt            # format code
make generate       # regenerate deepcopy methods for api types
make manifests      # regenerate CRD YAML manifests for api types
make all            # full pipeline: deps + generate + manifests + fmt + lint + test + build
make help           # show all targets
```

After any change to `apis/v1/*_types.go`, run `make generate manifests` and commit the regenerated `zz_generated.deepcopy.go` and `apis/v1/crds/*.yaml`. CI rejects PRs that forget this step.

## Related Projects

| Project | Role |
|---|---|
| [cocoon-operator](https://github.com/cocoonstack/cocoon-operator) | CocoonSet and Hibernation controllers |
| [cocoon-webhook](https://github.com/cocoonstack/cocoon-webhook) | Admission webhook for sticky scheduling |
| [vk-cocoon](https://github.com/cocoonstack/vk-cocoon) | Virtual kubelet provider |
| [cocoon-net](https://github.com/cocoonstack/cocoon-net) | Per-host VM networking |

## License

[MIT](LICENSE)
