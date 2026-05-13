package meta

const (
	// APIVersion is the apiVersion string for CocoonSet resources.
	APIVersion = "cocoonset.cocoonstack.io/v1"
	// KindCocoonSet is the kind string for CocoonSet resources.
	KindCocoonSet = "CocoonSet"

	// TolerationKey is the virtual-kubelet provider key used to gate cocoon pods onto vk-cocoon nodes.
	TolerationKey = "virtual-kubelet.io/provider"

	// LabelCocoonSet stamps a pod with its owning CocoonSet name.
	LabelCocoonSet = "cocoonset.cocoonstack.io/name"
	// LabelRole stamps a pod with its role (main / sub-agent / toolbox).
	LabelRole = "cocoonset.cocoonstack.io/role"
	// LabelSlot stamps a pod with its zero-based agent slot index.
	LabelSlot = "cocoonset.cocoonstack.io/slot"

	// LabelNodePool selects which cocoon node pool a pod should land on.
	LabelNodePool = "cocoonstack.io/pool"
	// DefaultNodePool is the pool name used when LabelNodePool is unset.
	DefaultNodePool = "default"

	// AnnotationMode declares the VM provisioning mode (clone / run / static).
	AnnotationMode = "cocoonset.cocoonstack.io/mode"
	// AnnotationImage carries the VM image reference.
	AnnotationImage = "cocoonset.cocoonstack.io/image"
	// AnnotationStorage carries the VM root volume size (resource.Quantity).
	AnnotationStorage = "cocoonset.cocoonstack.io/storage"
	// AnnotationManaged marks a VM as cocoon-managed ("true") versus user-managed/static.
	AnnotationManaged = "cocoonset.cocoonstack.io/managed"
	// AnnotationOS carries the guest OS family (linux / windows / android).
	AnnotationOS = "cocoonset.cocoonstack.io/os"
	// AnnotationSnapshotPolicy carries the per-pod snapshot policy.
	AnnotationSnapshotPolicy = "cocoonset.cocoonstack.io/snapshot-policy"
	// AnnotationNetwork carries the cluster network to attach the VM to.
	AnnotationNetwork = "cocoonset.cocoonstack.io/network"
	// AnnotationForcePull bypasses the image cache when set to "true".
	AnnotationForcePull = "cocoonset.cocoonstack.io/force-pull"
	// AnnotationCocoonSetGeneration carries the CocoonSet generation stamped at scheduling time.
	AnnotationCocoonSetGeneration = "cocoonset.cocoonstack.io/generation"

	// AnnotationVMID carries the runtime VM identifier vk-cocoon assigns after creation.
	AnnotationVMID = "vm.cocoonstack.io/id"
	// AnnotationVMName carries the deterministic VM name the operator builds from namespace/deployment/slot.
	AnnotationVMName = "vm.cocoonstack.io/name"
	// AnnotationIP carries the VM's primary IPv4 address.
	AnnotationIP = "vm.cocoonstack.io/ip"
	// AnnotationVNCPort carries the VM's VNC port when one is exposed.
	AnnotationVNCPort = "vm.cocoonstack.io/vnc-port"
	// AnnotationHibernate signals "hibernate this VM" when set to "true".
	AnnotationHibernate = "vm.cocoonstack.io/hibernate"
	// AnnotationForkFrom names a VM to fork the new VM from.
	AnnotationForkFrom = "vm.cocoonstack.io/fork-from"
	// AnnotationCloneFromDir names a host directory to clone the VM image from (vk-cocoon-specific).
	AnnotationCloneFromDir = "vm.cocoonstack.io/clone-from-dir"
	// AnnotationConnType overrides the connection protocol inferred from OS/runtime.
	AnnotationConnType = "vm.cocoonstack.io/conn-type"
	// AnnotationBackend selects the hypervisor backend (cloud-hypervisor / firecracker).
	AnnotationBackend = "vm.cocoonstack.io/backend"
	// AnnotationNoDirectIO disables O_DIRECT on writable disks when set to "true" (cloud-hypervisor only).
	AnnotationNoDirectIO = "vm.cocoonstack.io/no-direct-io"
	// AnnotationProbePort overrides the default ICMP readiness probe with a TCP port check.
	AnnotationProbePort = "vm.cocoonstack.io/probe-port"
	// AnnotationLifecycleState carries the vk-cocoon-reported lifecycle state.
	AnnotationLifecycleState = "vm.cocoonstack.io/lifecycle-state"
	// AnnotationLifecycleObservedGeneration carries the CocoonSet generation observed by vk-cocoon.
	AnnotationLifecycleObservedGeneration = "vm.cocoonstack.io/lifecycle-observed-generation"
	// AnnotationLifecycleStateMessage carries an optional message accompanying the lifecycle state.
	AnnotationLifecycleStateMessage = "vm.cocoonstack.io/lifecycle-state-message"

	// RoleMain identifies the main agent VM (slot 0).
	RoleMain = "main"
	// RoleSubAgent identifies a sub-agent VM (slot > 0).
	RoleSubAgent = "sub-agent"
	// RoleToolbox identifies a toolbox VM.
	RoleToolbox = "toolbox"

	// HibernateSnapshotTag names the snapshot tag used for hibernation.
	HibernateSnapshotTag = "hibernate"
	// DefaultSnapshotTag names the default snapshot tag.
	DefaultSnapshotTag = "latest"

	// annotationTrue is the canonical "true" string for bool-valued annotations.
	annotationTrue = "true"
)
