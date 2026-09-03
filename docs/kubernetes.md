# Kubernetes helpers

`k8s` collects the client bootstrap, patch, condition, environment, TLS, and
admission plumbing every cocoonstack controller would otherwise re-implement.

Import path: `github.com/cocoonstack/cocoon-common/k8s`.

## Client configuration

`k8s.LoadConfig()` resolves a `*rest.Config` through the standard deferred
loading rules, first match wins:

1. `$KUBECONFIG` — an `os.PathListSeparator` list is merged, as `kubectl` does
2. `~/.kube/config`
3. in-cluster config

It then replaces client-go's `5 QPS / 10 burst` defaults, which throttle a
reconciler long before the apiserver does:

| Variable | Default | Sets |
|---|---|---|
| `COCOON_K8S_QPS` | `50` | `rest.Config.QPS` |
| `COCOON_K8S_BURST` | `100` | `rest.Config.Burst` |

Both parse leniently — an unset or malformed value falls back to the default.

`k8s.NewClientset()` and `k8s.NewClientsetAndDynamic()` build clients on top of
`LoadConfig` for binaries that do not run a controller-runtime manager; the
two-client form shares one `rest.Config`.

## Patching

Two shapes, both built on `client.MergeFrom` with the pre-mutation snapshot
taken from the kubebuilder-generated typed `DeepCopy()`:

```go
k8s.PatchStatus(ctx, cli, cs, func(c *v1.CocoonSet) { c.Status.Phase = ... })
k8s.Patch(ctx, cli, pod, func(p *corev1.Pod) { ... })
```

For reconcilers that prefer the raw JSON merge-patch encoding,
`k8s.AnnotationsMergePatch` builds the body — pair it with
[`meta.LifecycleStatus.Annotations()`](meta.md#lifecycle-status),
where a nil value means "delete this key".

## Conditions

`k8s.NewReadyCondition(generation, status, reason, message)` builds the
canonical `Ready` condition (`k8s.ConditionTypeReady`) shared by every cocoon
CRD status block. It leaves `LastTransitionTime` zero so
`apimeta.SetStatusCondition` preserves the existing timestamp on a no-op
update.

`k8s.Eventf(rec, obj, eventType, reason, format, args...)` records through a
`record.EventRecorder` and is a no-op when the recorder is nil, so tests and
recorder-less builds stay silent without a guard at every call site.

## Environment and loops

| Helper | Behaviour |
|---|---|
| `EnvOrDefault(key, fallback)` | `os.Getenv`, falling back when unset or empty |
| `EnvBool(key, fallback)` | falls back on unset or unparseable |
| `EnvInt(key, fallback)` | falls back on unset or unparseable |
| `SleepCtx(ctx, d)` | sleeps for `d`; returns `false` when ctx fires first |
| `RunTicker(ctx, interval, fn)` | calls `fn` every `interval` until ctx is canceled |

`SleepCtx` returning a bool lets a retry loop exit without a second `select`;
`RunTicker` is its repeat-until-canceled counterpart, so background loops stop
open-coding a ticker plus `select`.

## TLS bring-up

```go
cert, source, err := k8s.LoadOrGenerateCert(ctx, certPath, keyPath, hostname, ip)
```

Loads a keypair from disk and falls back to an in-memory self-signed ECDSA
P-256 certificate when the paths are empty, the certificate file is missing,
or the certificate has expired. The returned `source` label (`disk <path>` or
`self-signed`) is meant for a startup log line.

`k8s.GenerateSelfSignedCert(hostname, ip)` exposes the fallback directly, and
`k8s.DetectNodeIP()` returns the first non-loopback IPv4 address or
`k8s.ErrNoNodeIP`. Detection never substitutes localhost on failure —
auto-substituting would mask a misconfigured network namespace, so the caller
picks the fallback.

## Admission scaffolding

`k8s/admission` carries the request loop shared by `cocoon-webhook` and any
future cocoonstack admission handler.

```go
import commonadmission "github.com/cocoonstack/cocoon-common/k8s/admission"

mux.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
    commonadmission.Serve(w, r, 0, func(ctx context.Context, rev *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
        if bad(rev) {
            return commonadmission.Deny("reason")
        }
        return commonadmission.Allow()
    })
})
```

`Serve` decodes the review, rejects a missing `request` with 400, dispatches,
copies the request UID onto the response, and writes the encoded review. A nil
handler return is normalised to `Allow()`. The body is capped at
`DefaultMaxBody` (10 MiB) when the `maxBytes` argument is not positive;
`Decode` is exported for handlers that need the review without the response
half.
