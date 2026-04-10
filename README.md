# cocoon-common

Shared Go packages for [cocoonstack](https://github.com/cocoonstack) services.

## Overview

- `apis/v1alpha1` -- typed CocoonSet and CocoonHibernation CRD Go types and generated CRD YAML manifests
- `meta` -- shared CRD identifiers, annotation/label/toleration keys, VM naming helpers, and the typed `VMSpec` / `VMRuntime` / `HibernateState` annotation contract
- `k8s` -- Kubernetes client config bootstrap with the standard kubeconfig fallback chain plus merge-patch helpers
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

### `apis/v1alpha1`

Typed Go definitions for the `cocoonset.cocoonstack.io/v1alpha1` API
group, plus the generated CRD YAML manifests under
`apis/v1alpha1/crds/`. The package ships:

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
| `cocoonset.cocoonstack.io/` | CocoonSet CRD group, Pod selector labels, and declarative fields mirrored from a CocoonSet spec onto a managed Pod | `cocoonset.cocoonstack.io/v1alpha1`, `name`, `role`, `slot`, `mode`, `image`, `os`, `storage`, `snapshot-policy`, `network`, `managed` |
| `vm.cocoonstack.io/` | Runtime state observed about the VM backing a Pod | `id`, `name`, `ip`, `vnc-port`, `hibernate`, `fork-from` |

For typed annotation access, prefer the `meta.VMSpec` / `meta.VMRuntime` / `meta.HibernateState` wrappers over raw map manipulation:

```go
// operator side: build the spec contract for vk-cocoon to consume
spec := meta.VMSpec{
    VMName:         "vk-prod-demo-0",
    Image:          "ghcr.io/cocoonstack/cocoon/ubuntu:24.04",
    Mode:           string(v1alpha1.AgentModeClone),
    OS:             string(v1alpha1.OSLinux),
    SnapshotPolicy: string(v1alpha1.SnapshotPolicyAlways),
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

After any change to `apis/v1alpha1/*_types.go`, run `make generate manifests` and commit the regenerated `zz_generated.deepcopy.go` and `apis/v1alpha1/crds/*.yaml`. CI rejects PRs that forget this step.

## Related Projects

| Project | Role |
|---|---|
| [cocoon-operator](https://github.com/cocoonstack/cocoon-operator) | CocoonSet and Hibernation controllers |
| [cocoon-webhook](https://github.com/cocoonstack/cocoon-webhook) | Admission webhook for sticky scheduling |
| [epoch](https://github.com/cocoonstack/epoch) | Snapshot registry and storage backend |
| [vk-cocoon](https://github.com/cocoonstack/vk-cocoon) | Virtual kubelet provider |

## License

[MIT](LICENSE)
