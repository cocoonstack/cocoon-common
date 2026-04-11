# cocoon-common

Shared Go packages for [cocoonstack](https://github.com/cocoonstack) services.

## Overview

- `apis/v1` -- typed CocoonSet and CocoonHibernation CRD Go types and generated CRD YAML manifests
- `meta` -- shared CRD identifiers, annotation/label/toleration keys, VM naming helpers, the typed `VMSpec` / `VMRuntime` / `HibernateState` annotation contract, and pod-state helpers (`IsPodReady`, `IsPodTerminal`, `IsContainerRunning`, `IsWindowsPod`, `PodKey`, `PodNodePool`) every cocoon component shares
- `k8s` -- Kubernetes client config bootstrap with the standard kubeconfig fallback chain, merge-patch helpers, env/duration/sleep helpers (`EnvOrDefault`, `EnvDuration`, `EnvBool`, `SleepCtx`), unstructured decoder, and TLS helpers (`LoadOrGenerateCert`, `GenerateSelfSignedCert`, `DetectNodeIP`)
- `k8s/admission` -- shared admission-webhook scaffolding (`Allow` / `Deny` responses, `Decode` / `Serve` request loop, RFC 6902 `JSONPatchOp` + `EscapeJSONPointer` helpers) used by `cocoon-webhook` and reusable by any future cocoonstack admission handler
- `log` -- common log setup for cocoonstack binaries using `projecteru2/core/log`

This repository keeps cross-project contracts in one place instead of re-exporting them from `cocoon-operator`. `cocoon-operator`, `cocoon-webhook`, and `vk-cocoon` all consume the same package set directly.

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

Typed Go definitions for the `cocoonset.cocoonstack.io/v1` API
group, plus the generated CRD YAML manifests under
`apis/v1/crds/`. The package ships:

- `CocoonSet` -- declarative spec for an agent cluster (main + sub-agents + toolboxes)
- `CocoonHibernation` -- per-pod hibernate / wake request

Downstream operators import these via `go list -m` and copy the CRD
YAML into their own kustomize tree (see `make import-crds` in
cocoon-operator). Regenerate via `make generate manifests` after any
type change.

### `meta`

Use `meta` for:

- Cocoon annotation, label, and CRD identifier constants
- stable VM naming helpers
- slot extraction and role inference
- toolbox connection type detection

#### Identifier namespaces

All identifiers live under two cocoonstack.io subdomains:

| Prefix | Used for | Examples |
|---|---|---|
| `cocoonset.cocoonstack.io/` | CocoonSet CRD group, Pod selector labels, and declarative fields mirrored from a CocoonSet spec onto a managed Pod | `cocoonset.cocoonstack.io/v1`, `name`, `role`, `slot`, `mode`, `image`, `os`, `storage`, `snapshot-policy`, `network`, `managed` |
| `vm.cocoonstack.io/` | Runtime state observed about the VM backing a Pod | `id`, `name`, `ip`, `vnc-port`, `hibernate`, `fork-from` |

For typed annotation access, prefer the `meta.VMSpec` / `meta.VMRuntime` / `meta.HibernateState` wrappers over raw map manipulation:

```go
// operator side: build the spec contract for vk-cocoon to consume
spec := meta.VMSpec{
    VMName:         "vk-prod-demo-0",
    Image:          "ghcr.io/cocoonstack/cocoon/ubuntu:24.04",
    Mode:           string(v1.AgentModeClone),
    OS:             string(v1.OSLinux),
    SnapshotPolicy: string(v1.SnapshotPolicyAlways),
    Managed:        true,
}
spec.Apply(pod)

// vk-cocoon side: read it back and write the runtime contract
runtime := meta.VMRuntime{VMID: vmID, IP: ip, VNCPort: vncPort}
runtime.Apply(pod)

// hibernate / wake
meta.HibernateState(true).Apply(pod)
```

`meta.HibernateSnapshotTag` (`"hibernate"`) is the OCI tag used both
when vk-cocoon pushes a hibernation snapshot to epoch and when the
operator probes whether a hibernation has completed.

### `k8s`

Use `k8s.LoadConfig()` to resolve cluster configuration from:

1. `KUBECONFIG`
2. `~/.kube/config`
3. in-cluster config

Other helpers in this package:

- `k8s.EnvOrDefault`, `k8s.EnvDuration`, `k8s.EnvBool` -- lenient env-var parsing that falls back to the supplied default on unset / malformed values.
- `k8s.SleepCtx(ctx, d)` -- context-aware sleep; returns `false` when the context fires first so callers can exit retry loops without a second `select`.
- `k8s.LoadOrGenerateCert` / `k8s.GenerateSelfSignedCert` / `k8s.DetectNodeIP` -- TLS bring-up helpers used by `vk-cocoon` and reusable by any cocoonstack HTTP server that needs a dev-time self-signed fallback.
- `k8s.StatusMergePatch` / `k8s.AnnotationsMergePatch` -- merge-patch builders used by reconcilers that prefer the JSON merge-patch encoding over `client.MergeFrom`.
- `k8s.PatchStatus[T]` -- generic `client.MergeFrom` patch for the `/status` subresource; captures the pre-mutation snapshot via the kubebuilder-generated typed `DeepCopy()` so callers skip the boilerplate.
- `k8s.PatchHibernateState` -- pod-level hibernate annotation patch that short-circuits when the pod already carries the desired state, safe to call unconditionally in a reconcile loop.
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

`commonadmission.JSONPatchOp`, `commonadmission.MarshalPatch`, and `commonadmission.EscapeJSONPointer` cover the RFC 6902 patch flow for mutating webhooks.

### `log`

Use `log.Setup(ctx, envVar)` to initialize the shared logger from an environment variable, defaulting to `info`.

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
| [epoch](https://github.com/cocoonstack/epoch) | Snapshot registry and storage backend |
| [vk-cocoon](https://github.com/cocoonstack/vk-cocoon) | Virtual kubelet provider |

## License

[MIT](LICENSE)
