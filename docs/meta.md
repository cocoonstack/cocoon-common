# Metadata contract

`meta` owns the shared identifiers cocoon components stamp on Kubernetes
objects, plus the typed wrappers that read and write them. Components may
also define private keys for their own reconciliation state.

Import path: `github.com/cocoonstack/cocoon-common/meta`.

## Identifier namespaces

All identifiers live under three `cocoonstack.io` prefixes. The prefix tells
you who owns the value.

| Prefix | Owner and use | Keys |
|---|---|---|
| `cocoonset.cocoonstack.io/` | CocoonSet CRD group, pod selector labels, and CocoonSet-level fields the operator mirrors onto a managed pod | `name`, `role`, `slot`, `mode`, `image`, `os`, `storage`, `snapshot-policy`, `network`, `managed`, `force-pull`, `generation`, `hibernated-on-node` |
| `vm.cocoonstack.io/` | VM-instance metadata — observed runtime state plus the per-VM spec the operator hands to vk-cocoon | `id`, `name`, `ip`, `vnc-port`, `hibernate`, `restore-from-hibernate`, `keep-snapshot-on-delete`, `fork-from`, `clone-from-dir`, `conn-type`, `backend`, `no-direct-io`, `probe-port`, `lifecycle-state`, `lifecycle-observed-generation`, `lifecycle-state-message` |
| `cocoonstack.io/` | Node labels vk-cocoon stamps on its virtual node and the operator selects on | `pool`, `snapshot-cpu-class` |

`meta.KindCocoonSet` and `meta.TolerationKey`
(`virtual-kubelet.io/provider`) complete the set.

## Typed annotation wrappers

Prefer these over raw map manipulation — they are the only place the key/value
encoding lives.

```go
// Managed=true: vk-cocoon owns lifecycle; false: adopt a pre-assigned VM.
spec := meta.VMSpec{
    VMName:         "vk-prod-demo-0",
    Image:          "ghcr.io/cocoonstack/cocoon/ubuntu:24.04",
    Mode:           string(v1.AgentModeRun),
    OS:             string(v1.OSLinux),
    Backend:        string(v1.BackendFirecracker),
    SnapshotPolicy: string(v1.SnapshotPolicyAlways),
    Managed:        true,
    ForcePull:      true,
    ProbePort:      "22",
}
spec.Apply(pod)

// vk-cocoon side: write runtime state back onto the pod.
meta.VMRuntime{VMID: vmID, IP: ip}.Apply(pod)

// hibernate / wake
meta.HibernateState(true).Apply(pod)
```

`VMSpec.Apply` and `VMRuntime.Apply` skip empty string fields, and runtime ports
are emitted only when positive. Spec booleans are written only when true;
false does not clear an existing annotation. Clearing runtime values requires
an explicit annotation patch. `HibernateState(false).Apply` removes its key.
`LifecycleStatus.Apply` always writes state and observed generation, and
removes the message key when `Message` is empty.
`ParseVMSpec` / `ParseVMRuntime` / `ReadHibernateState` read them back.

`meta.FromAgentSpec` and `meta.FromToolboxSpec` build a `VMSpec` straight from
the CRD types, resolving each enum that declares a `Default()` through it;
`ConnType` has none and is passed through as-is.

## VM naming and roles

```go
meta.VMNameForDeployment(ns, cocoonSet, slot)  // "vk-<ns>.<set>-<slot>"
meta.ToolboxPodName(cocoonSet, toolbox)        // "<set>-<toolbox>"
meta.VMNameForPod(ns, podName)                 // "vk-<ns>.<pod>"
meta.AgentVMNamePrefix(ns, cocoonSet)          // "vk-<ns>.<set>-"
meta.ExtractAgentSlot(ns, cocoonSet, vmName)   // slot, or -1 for a toolbox
meta.InferRoleFromAgentSlot(slot)              // main / sub-agent / toolbox
meta.RoleForPod(pod, vmName)                   // owner ref + name → role
```

The namespace never contains a dot, so `vk-<ns>.<pod>` decodes uniquely and
two pods never share a VM name (`team-a/dev-0` is `vk-team-a.dev-0`, `team/a-dev-0`
is `vk-team.a-dev-0`). `ExtractAgentSlot` rejects any suffix containing a dash, so
a toolbox named `app-0` (VM name `vk-ns.set-app-0`) can never be misread as agent
slot 0. A toolbox VM is `VMNameForPod(ns, ToolboxPodName(set, toolbox))`.

A pulled hibernate snapshot is imported as `<vm>` + `meta.HibernateImportSuffix`
(`-hibernate-import`) so it never collides with the live VM; the webhook caps
non-macOS VM names at 63 minus that suffix so the import name still fits
cocoon's 63-character limit.

When a Pod is available, use `RoleForPod` to combine its CocoonSet ownership
with the VM name. Cleanup paths that no longer have a Pod can use
`ExtractAgentSlot` with the owning CocoonSet's namespace and name.

## Snapshot contract

Two tag constants anchor the cross-component contract:

- `meta.HibernateSnapshotTag` (`hibernate`) — the tag vk-cocoon pushes a
  hibernation snapshot under, and the tag the operator probes to detect that a
  hibernation completed.
- `meta.DefaultSnapshotTag` (`latest`) — the tag vk-cocoon publishes routine
  VM snapshots under at pod-delete time when the snapshot policy permits it.

`meta.ShouldSnapshotVM(spec, role)` returns true for `always`, true only for
the main role under `main-only`, and false for `never`. It checks the policy
and role, not `Managed`; vk-cocoon checks lifecycle ownership separately.

After all owned Pods are gone, cocoon-operator deletes their `hibernate` tags.
It keeps `latest` tags selected by the snapshot policy for later reuse:
`always` keeps every role, `main-only` keeps only the main agent, and `never`
keeps none. Its cleanup derives the role from the stored VM name because the
Pods have already been deleted.

`meta.LabelSnapshotCompatibilityClass`
(`cocoonstack.io/snapshot-cpu-class`) closes the same loop for placement:
`CocoonSetSpec.SnapshotCompatibilityClass` names a certified guest-visible CPU
ABI, cocoon-operator renders it as a hard node selector on every managed pod,
and vk-cocoon publishes the label on nodes configured with that class and
refuses any classified pod it cannot serve. It is independent from
`LabelNodePool`, so several workload pools can share one snapshot class.

`meta.AnnotationKeepSnapshotOnDelete` marks a pod delete as a scheduling-seat
release rather than a teardown: vk-cocoon keeps the node-local snapshot as a
warm-wake cache instead of dropping it with the VM. It is best-effort by
contract — a lost flag costs the wake a registry pull, never correctness.

`meta.AnnotationHibernatedOnNode` records the main agent's node at
release-policy suspend; wake uses it as a preferred-affinity hint for a warm
restore.

## Lifecycle status

`meta.LifecycleStatus` is the typed contract for the annotation triple
vk-cocoon writes (`lifecycle-state`, `lifecycle-observed-generation`,
`lifecycle-state-message`):

```go
status := meta.LifecycleStatus{
    State:              meta.LifecycleStateReady,
    ObservedGeneration: meta.ReadCocoonSetGeneration(pod),
}

// In-memory: mutate the pod we already hold.
status.Apply(pod)

// Wire: Annotations() returns the key/value map (a nil value deletes the key);
// wrap it with k8s.AnnotationsMergePatch for an apiserver merge-patch body.
patch, _ := k8s.AnnotationsMergePatch(status.Annotations())
```

States are `creating`, `ready`, `hibernating`, `hibernated`, `failed`; a
client waits for one of `ready`, `hibernated` or `failed`. An empty message
clears the annotation, so
a stale failure reason cannot tail into the next lifecycle.
`LifecycleStatus.Snapshot()` returns a NUL-separated comparison key for
change detection.

cocoon-operator stamps the owning CocoonSet's `metadata.generation` onto the
pod via `meta.StampCocoonSetGeneration` so vk-cocoon can echo it back as
`lifecycle-observed-generation`. Counter-based completion lets a client tell
"the operation I asked for finished" from "an older completion is still being
reported", with no dependence on wall-clock skew.

## Pod helpers

| Helper | Meaning |
|---|---|
| `IsPodReady(pod)` | A `PodReady=True` condition exists |
| `IsPodTerminal(pod)` | Phase is `PodFailed`; `PodSucceeded` is not included |
| `IsContainerRunning(pod)` | Any container status reports `Running` |
| `VMLive(pod)` | A container reports `Running` and the VMID annotation is non-empty; this does not test readiness or contact the VM |
| `PodKey(ns, name)` | `namespace/name` |
| `PodNodePool(pod)` | First non-empty pool in nodeSelector, labels, annotations, then `DefaultNodePool` |

`HasCocoonTolerationKey` and `IsOwnedByCocoonSet` / `CocoonSetOwnerName` cover
the admission-side ownership checks. The toleration check matches on key
alone; operator, value, and effect are ignored so every toleration bearing the
key is included in the gate. The owner helpers match the owner reference's
Kind and return its name; they do not authenticate who created the Pod.

`ConnectionType(osType, hasVNCPort, override)` resolves the connection
protocol: an explicit override wins, then a VNC port, then the OS family
(android→adb, windows→rdp, otherwise ssh).
