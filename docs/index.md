# cocoon-common

The contract repository for the [cocoonstack](https://github.com/cocoonstack)
MicroVM platform. It supplies CRD types, pod metadata, OCI snapshot formats,
and runtime helpers used by cocoon-operator, cocoon-webhook, vk-cocoon, and
cocoon-net. Each consumer imports the packages it needs and pins the module
version independently.

```
apis/v1     CocoonSet + CocoonHibernation types, generated CRD YAML
meta        annotation/label keys and the typed wrappers over them
k8s         client bootstrap, patch helpers, TLS, admission scaffolding
snapshot    VM snapshot push/pull over OCI (v1 and v2 wire formats)
oci         the Registry interface + its standard-OCI implementation
manifest    OCI manifest types, media types, artifact classification
ociutil     reference parsing and size checks for digest-verified blob streams
cloudimg    cloud-image disk streaming out of a registry
httpx, log  HTTP server lifecycle and logger bootstrap
```

The dependency direction is one-way: cocoon-common depends on none of its
consumers, and its own packages form a DAG (`oci → snapshot → manifest,
ociutil`; `cloudimg → manifest, ociutil`; `meta → apis/v1`; `k8s` imports
nothing else in the module).

## Guides

- [API types](apis.md) — the `cocoonset.cocoonstack.io/v1` group, enum
  defaults, the CEL rules that ship inside the CRDs, and the regeneration
  workflow
- [Metadata contract](meta.md) — the three identifier namespaces, the typed
  annotation wrappers, VM naming and role inference, and the snapshot /
  hibernation / lifecycle contracts shared across components
- [Kubernetes helpers](kubernetes.md) — client config and rate limits, patch
  builders, conditions, TLS bring-up, and the admission-webhook scaffolding
- [Registry and snapshots](registry.md) — the artifact model, the v1 and v2
  snapshot wire formats, push and pull tuning knobs, and the media-type
  vocabulary
- [Runtime helpers](runtime.md) — HTTP server lifecycle and logger setup

## Repository

The Makefile's `deps` target detects Go workspace mode and skips `go mod tidy`
when a workspace is active. Sibling checkouts then resolve through `go.work`;
use `GOWORK=off` to check a consumer against its committed module versions.
See [downstream consumption](apis.md#downstream-consumption) for dependency
and CRD updates.

Source and issue tracker:
[github.com/cocoonstack/cocoon-common](https://github.com/cocoonstack/cocoon-common).
Part of the [cocoonstack](https://cocoonstack.github.io/) MicroVM platform.
