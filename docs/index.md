# cocoon-common

The contract repository for the [cocoonstack](https://github.com/cocoonstack)
MicroVM platform. Every Kubernetes-side component — cocoon-operator,
cocoon-webhook, vk-cocoon, cocoon-net — imports these packages instead of
re-declaring the CRD types, the pod annotation keys, or the OCI snapshot wire
format, so producer and consumer cannot drift.

```
apis/v1     CocoonSet + CocoonHibernation types, generated CRD YAML
meta        annotation/label keys and the typed wrappers over them
k8s         client bootstrap, patch helpers, TLS, admission scaffolding
snapshot    VM snapshot push/pull over OCI (v1 and v2 wire formats)
oci         the Registry interface + its standard-OCI implementation
manifest    OCI manifest types, media types, artifact classification
ociutil     reference parsing and digest/size-verified blob copies
cloudimg    cloud-image (qcow2) streaming out of a registry
httpx, log  HTTP server lifecycle and logger bootstrap
```

The dependency direction is one-way: cocoon-common depends on none of its
consumers, and its own packages form a DAG (`oci → snapshot → manifest,
ociutil`; `k8s → meta → apis/v1`).

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

Source and issue tracker:
[github.com/cocoonstack/cocoon-common](https://github.com/cocoonstack/cocoon-common).
Part of the [cocoonstack](https://cocoonstack.github.io/) MicroVM platform.
