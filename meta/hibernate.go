package meta

import (
	corev1 "k8s.io/api/core/v1"
)

// HibernateState is the typed contract for the hibernate annotation.
type HibernateState bool

// Apply writes HibernateState into pod annotations. False removes the annotation entirely.
func (s HibernateState) Apply(pod *corev1.Pod) {
	if !bool(s) {
		delete(pod.Annotations, AnnotationHibernate)
		return
	}
	a := ensurePodAnnotations(pod)
	a[AnnotationHibernate] = annotationTrue
}

// ReadHibernateState reads the hibernate annotation from a pod.
func ReadHibernateState(pod *corev1.Pod) HibernateState {
	return HibernateState(pod.Annotations[AnnotationHibernate] == annotationTrue)
}

// ReadRestoreFromHibernate reports whether the pod is flagged to restore its VM
// from the :hibernate snapshot instead of cloning from the base image.
func ReadRestoreFromHibernate(pod *corev1.Pod) bool {
	return pod.Annotations[AnnotationRestoreFromHibernate] == annotationTrue
}

// MarkRestoreFromHibernate flags a pod to restore its VM from the :hibernate
// snapshot instead of cloning from the base image.
func MarkRestoreFromHibernate(pod *corev1.Pod) {
	a := ensurePodAnnotations(pod)
	a[AnnotationRestoreFromHibernate] = annotationTrue
}

// ReadKeepSnapshotOnDelete reports whether the pod's deletion is flagged as a seat release.
func ReadKeepSnapshotOnDelete(pod *corev1.Pod) bool {
	return pod.Annotations[AnnotationKeepSnapshotOnDelete] == annotationTrue
}

// MarkKeepSnapshotOnDelete flags a pod's deletion as a seat release.
func MarkKeepSnapshotOnDelete(pod *corev1.Pod) {
	a := ensurePodAnnotations(pod)
	a[AnnotationKeepSnapshotOnDelete] = annotationTrue
}
