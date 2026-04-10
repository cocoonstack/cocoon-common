# cocoon-common

Shared Go packages for [cocoonstack](https://github.com/cocoonstack) services.

## Overview

- `meta` -- shared annotation keys, label keys, toleration keys, and VM naming rules
- `k8s` -- Kubernetes client config bootstrap with the standard kubeconfig fallback chain
- `log` -- common log setup for cocoonstack binaries using `projecteru2/core/log`

This repository keeps cross-project contracts in one place instead of re-exporting them from `cocoon-operator`. `cocoon-operator`, `cocoon-webhook`, `glance`, and `vk-cocoon` all consume the same package set directly.

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

- Cocoon annotation and label constants
- stable VM naming helpers
- slot extraction and role inference
- toolbox connection type detection

#### Annotation namespace

Annotation keys live under two cocoonstack.io subdomains, split by what they describe:

| Prefix | Meaning | Examples |
|---|---|---|
| `cocoonset.cocoonstack.io/` | Declarative fields mirrored from a CocoonSet spec onto a managed Pod | `mode`, `image`, `os`, `storage`, `snapshot-policy`, `network`, `managed` |
| `vm.cocoonstack.io/` | Runtime state observed about the VM backing a Pod | `id`, `name`, `ip`, `vnc-port`, `hibernate`, `fork-from` |

The legacy `cocoon.cis/*` prefix is being phased out. During the migration window writers dual-emit both the canonical and the legacy key on every reconcile, and readers fall through to the legacy key automatically. Use the helpers below rather than touching the maps directly:

| Helper | Use |
|---|---|
| `meta.ReadAnnotation(annotations, meta.AnnotationFoo)` | Read; checks the canonical key first, falls back to `cocoon.cis/foo` |
| `meta.WriteAnnotation(m, meta.AnnotationFoo, value)` | Write a single annotation to both the canonical and legacy keys |
| `meta.AddLegacyAnnotations(m)` | Walk a freshly built map literal and mirror every canonical key to its legacy equivalent in one shot |
| `meta.LegacyAnnotationKey(meta.AnnotationFoo)` | Look up the legacy key for a single canonical key — useful when building merge patches that need both keys in the same PATCH |

The CRD API group (`cocoon.cis/v1alpha1`) and Pod selector labels (`cocoon.cis/cocoonset` etc.) are **not** renamed in this round. Renaming the API group requires migrating every existing CocoonSet CR; renaming the selector labels requires a coordinated cutover so in-flight Pods are not orphaned. Both will move under cocoonstack.io in dedicated follow-ups.

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
