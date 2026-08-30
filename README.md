# cocoon-common

The contract repository for the [cocoonstack](https://github.com/cocoonstack)
MicroVM platform: the CRD Go types, the pod annotation contract, and the OCI
snapshot wire format that cocoon-operator, cocoon-webhook, vk-cocoon, and
cocoon-net all import instead of re-declaring. Cross-repo shared code lives
here and nowhere else, so producer and consumer cannot drift.

**Documentation: [cocoonstack.github.io/cocoon-common](https://cocoonstack.github.io/cocoon-common/)** (source in [`docs/`](docs/)).

## Documentation

- [API types](docs/apis.md) — the `cocoonset.cocoonstack.io/v1` group, enum defaults, the CEL rules that ship inside the CRDs, and the regeneration workflow
- [Metadata contract](docs/meta.md) — the three identifier namespaces, the typed annotation wrappers, VM naming and role inference, and the snapshot / hibernation / lifecycle contracts
- [Kubernetes helpers](docs/kubernetes.md) — client config and rate limits, patch builders, conditions, TLS bring-up, and admission-webhook scaffolding
- [Registry and snapshots](docs/registry.md) — the artifact model, the v1 and v2 snapshot wire formats, push and pull tuning knobs, and the media-type vocabulary
- [Runtime helpers](docs/runtime.md) — HTTP server lifecycle and logger setup

## Packages

| Package | Contract |
|---|---|
| `apis/v1` | Typed `CocoonSet` / `CocoonHibernation` CRDs plus the generated YAML under `apis/v1/crds/` |
| `meta` | Annotation, label, and CRD identifier keys, VM naming, and the typed `VMSpec` / `VMRuntime` / `HibernateState` / `LifecycleStatus` wrappers over them |
| `k8s` | Client bootstrap, merge-patch helpers, conditions, env/sleep helpers, and TLS bring-up |
| `k8s/admission` | Admission-webhook scaffolding — `Allow` / `Deny`, `Decode` / `Serve` |
| `snapshot` | Push cocoon VM snapshots to an OCI registry and stream them back, including the chunked/zstd v2 wire format |
| `oci` | The `Registry` interface every consumer codes against, plus the standard-OCI implementation |
| `manifest` | OCI manifest / descriptor types, the cocoon snapshot config, and media-type classification |
| `ociutil` | Reference parsing and digest/size-verified blob copies |
| `cloudimg` | Stream a cocoonstack cloud-image (qcow2) artifact out of a registry |
| `httpx` | HTTP server bootstrap and coordinated graceful shutdown |
| `log` | Shared logger setup over `projecteru2/core/log` |

## Quick start

```bash
go get github.com/cocoonstack/cocoon-common@latest
```

```go
import (
    v1 "github.com/cocoonstack/cocoon-common/apis/v1"
    "github.com/cocoonstack/cocoon-common/meta"
)

// The operator stamps the VM spec a pod's provider will read back.
meta.FromAgentSpec(cs.Spec.Agent, vmName, cs.Spec.SnapshotPolicy, "").Apply(pod)

// vk-cocoon reports lifecycle state on the same pod.
meta.LifecycleStatus{
    State:              meta.LifecycleStateReady,
    ObservedGeneration: meta.ReadCocoonSetGeneration(pod),
}.Apply(pod)
```

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

After any change to `apis/v1/*_types.go`, run `make generate manifests` and
commit the regenerated `zz_generated.deepcopy.go` and `apis/v1/crds/*.yaml`.
CI re-runs both and fails on a dirty tree.

The Makefile detects Go workspace mode (`go env GOWORK`) and skips
`go mod tidy` when active, so sibling checkouts resolve through `go.work`
without forcing a release of this module.

## Related projects

| Project | Role |
|---|---|
| [cocoon-operator](https://github.com/cocoonstack/cocoon-operator) | CocoonSet and CocoonHibernation controllers |
| [cocoon-webhook](https://github.com/cocoonstack/cocoon-webhook) | Admission webhook for sticky scheduling |
| [vk-cocoon](https://github.com/cocoonstack/vk-cocoon) | Virtual kubelet provider |
| [cocoon-net](https://github.com/cocoonstack/cocoon-net) | Per-host VM networking |

## License

[MIT](LICENSE)
