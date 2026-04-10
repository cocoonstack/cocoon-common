# cocoon-common

Shared Go packages for [cocoonstack](https://github.com/cocoonstack) services.

## Overview

- `meta` -- shared CRD identifiers, annotation/label/toleration keys, and VM naming rules
- `k8s` -- Kubernetes client config bootstrap with the standard kubeconfig fallback chain
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

Annotation reads and writes go through plain map access (`pod.Annotations[meta.AnnotationFoo]`); the package does not ship annotation compatibility shims.

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
make lint           # run golangci-lint
make fmt            # format code
make help           # show all targets
```

## Related Projects

| Project | Role |
|---|---|
| [cocoon-operator](https://github.com/cocoonstack/cocoon-operator) | CocoonSet and Hibernation controllers |
| [cocoon-webhook](https://github.com/cocoonstack/cocoon-webhook) | Admission webhook for sticky scheduling |
| [epoch](https://github.com/cocoonstack/epoch) | Snapshot registry and storage backend |
| [vk-cocoon](https://github.com/cocoonstack/vk-cocoon) | Virtual kubelet provider |

## License

[MIT](LICENSE)
