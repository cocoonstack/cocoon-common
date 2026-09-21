# cocoon-common

The contract repository for the [cocoonstack](https://github.com/cocoonstack)
MicroVM platform. It provides shared CRD Go types, pod metadata, OCI snapshot
formats, and runtime helpers for cocoon-operator, cocoon-webhook, vk-cocoon,
and cocoon-net in one Go module.

## Documentation

Published at [cocoonstack.github.io/cocoon-common](https://cocoonstack.github.io/cocoon-common/), with source in [`docs/`](docs/).

- [API types](docs/apis.md) — the `cocoonset.cocoonstack.io/v1` group, enum defaults, the CEL rules that ship inside the CRDs, and the regeneration workflow
- [Metadata contract](docs/meta.md) — the three identifier namespaces, the typed annotation wrappers, VM naming and role inference, and the snapshot / hibernation / lifecycle contracts
- [Kubernetes helpers](docs/kubernetes.md) — client config and rate limits, patch builders, conditions, TLS bring-up, and admission-webhook scaffolding
- [Registry and snapshots](docs/registry.md) — the artifact model, the v1 and v2 snapshot wire formats, push and pull tuning knobs, and the media-type vocabulary
- [Runtime helpers](docs/runtime.md) — HTTP server lifecycle and logger setup

## Architecture

| Package | Contract |
|---|---|
| `apis/v1` | Typed `CocoonSet` / `CocoonHibernation` CRDs plus the generated YAML under `apis/v1/crds/` |
| `meta` | Annotation, label, and CRD identifier keys, VM naming, and the typed `VMSpec` / `VMRuntime` / `HibernateState` / `LifecycleStatus` wrappers over them |
| `k8s` | Client bootstrap, merge-patch helpers, conditions, env/sleep helpers, and TLS bring-up |
| `k8s/admission` | Admission-webhook scaffolding — `Allow` / `Deny`, `Decode` / `Serve` |
| `snapshot` | Push cocoon VM snapshots to an OCI registry and stream them back, including the chunked/zstd v2 wire format |
| `oci` | The `Registry` interface every consumer codes against, plus the standard-OCI implementation |
| `manifest` | OCI manifest / descriptor types, the cocoon snapshot config, and media-type classification |
| `ociutil` | Reference parsing and size checks for digest-verified blob streams |
| `cloudimg` | Stream a cocoonstack cloud-image (qcow2 or raw) artifact out of a registry |
| `httpx` | HTTP server bootstrap and coordinated graceful shutdown |
| `log` | Shared logger setup over `projecteru2/core/log` |

## Quick start

```bash
go get github.com/cocoonstack/cocoon-common@latest
```

Given a CocoonSet `cs`, its Pod `pod`, and the VM name `vmName`:

```go
import "github.com/cocoonstack/cocoon-common/meta"

// The operator stamps the VM spec a pod's provider will read back.
meta.FromAgentSpec(cs.Spec.Agent, vmName, cs.Spec.SnapshotPolicy, "").Apply(pod)

// vk-cocoon reports lifecycle state on the same pod.
meta.LifecycleStatus{
    State:              meta.LifecycleStateReady,
    ObservedGeneration: meta.ReadCocoonSetGeneration(pod),
}.Apply(pod)
```

## Related projects

- [cocoon-operator](https://github.com/cocoonstack/cocoon-operator) — CocoonSet and CocoonHibernation controllers
- [cocoon-webhook](https://github.com/cocoonstack/cocoon-webhook) — Admission checks for Cocoon workloads and lifecycle requests
- [vk-cocoon](https://github.com/cocoonstack/vk-cocoon) — Virtual kubelet provider
- [cocoon-net](https://github.com/cocoonstack/cocoon-net) — Per-host VM networking

## Development

```bash
make build          # build all packages
make test           # vet + race-detected tests with coverage
make lint           # golangci-lint on linux + darwin
make fmt            # gofumpt + goimports
make generate       # regenerate deepcopy methods for api types
make manifests      # regenerate CRD YAML manifests for api types
make all            # deps + generate + manifests + fmt + lint + test + build
make help           # show all targets
```

## License

[MIT](LICENSE)
