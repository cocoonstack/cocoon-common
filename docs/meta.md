# Metadata contract

`meta` owns every identifier cocoon components stamp on a Kubernetes object,
plus the typed wrappers that read and write them. Nothing else in the platform
declares these strings.

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

`Apply` skips empty fields, so it can never clear a value another writer set.
The exceptions are deletes by design: `HibernateState(false).Apply` drops its
key, and `LifecycleStatus.Apply` drops the message key when `Message` is empty.
`ParseVMSpec` / `ParseVMRuntime` / `ReadHibernateState` read them back.

`meta.FromAgentSpec` and `meta.FromToolboxSpec` build a `VMSpec` straight from
the CRD types, resolving each enum that declares a `Default()` through it;
`ConnType` has none and is passed through as-is.

## VM naming and roles

```go
meta.VMNameForDeployment(ns, cocoonSet, slot)  // "vk-<ns>-<set>-<slot>"
meta.VMNameForPod(ns, podName)                 // "vk-<ns>-<pod>"
meta.AgentVMNamePrefix(ns, cocoonSet)          // "vk-<ns>-<set>-"
meta.ExtractAgentSlot(ns, cocoonSet, vmName)   // slot, or -1 for a toolbox
meta.InferRoleFromAgentSlot(slot)              // main / sub-agent / toolbox
meta.RoleForPod(pod, vmName)                   // owner ref + name → role
```

`ExtractAgentSlot` rejects any suffix containing a dash, so a toolbox named
`app-0` (VM name `vk-ns-set-app-0`) can never be misread as agent slot 0.

Role always comes from the pod's CocoonSet ownership via `RoleForPod`, never
from parsing a VM-name suffix at a call site.

## Snapshot contract

Two tag constants anchor the cross-component contract:

- `meta.HibernateSnapshotTag` (`hibernate`) — the tag vk-cocoon pushes a
  hibernation snapshot under, and the tag the operator probes to detect that a
  hibernation completed.
- `meta.DefaultSnapshotTag` (`latest`) — the tag vk-cocoon publishes routine
  VM snapshots under at pod-delete time, and the tag cocoon-operator garbage
  collects when a CocoonSet is deleted.

`meta.ShouldSnapshotVM(spec, role)` is the single shared decoder for the
`SnapshotPolicy` × role decision. vk-cocoon asks it on the producer side
("should I push this VM?") and cocoon-operator on the GC side ("should I
delete this tag?"), so the two cannot drift — under `main-only` both agree
that only the main agent is touched.

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

States are `creating`, `ready`, `hibernating`, `hibernated`, `failed`;
`LifecycleState.IsTerminal` reports the three a client would wait for
(`ready`, `hibernated`, `failed`). An empty message clears the annotation, so
a stale failure reason cannot tail into the next lifecycle.
`LifecycleStatus.Snapshot()` returns a NUL-separated comparison key for
change detection.

cocoon-operator stamps the owning CocoonSet's `metadata.generation` onto the
pod via `meta.StampCocoonSetGeneration` so vk-cocoon can echo it back as
`lifecycle-observed-generation`. Counter-based completion lets a client tell
"the operation I asked for finished" from "an older completion is still being
reported", with no dependence on wall-clock skew.

## Pod helpers

`IsPodReady`, `IsPodTerminal`, `IsContainerRunning`, `PodKey(ns, name)`, and
`PodNodePool(pod)` — the last resolving the pool from `nodeSelector`, then
labels, then annotations, then `DefaultNodePool`.

`HasCocoonTolerationKey` and `IsOwnedByCocoonSet` / `CocoonSetOwnerName` cover
the admission-side ownership checks. The toleration check matches on key
alone; operator, value, and effect are deliberately ignored so the
cocoon-webhook gate stays permissive.

`ConnectionType(osType, hasVNCPort, override)` resolves the connection
protocol: an explicit override wins, then a VNC port, then the OS family
(android→adb, windows→rdp, otherwise ssh).
