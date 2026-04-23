package meta

import (
	corev1 "k8s.io/api/core/v1"
)

// HibernateState is the typed contract for the hibernate annotation.
type HibernateState bool

// Apply writes HibernateState into pod annotations. False removes the annotation entirely.
func (s HibernateState) Apply(pod *corev1.Pod) {
	if pod == nil {
		return
	}
	if !bool(s) {
		delete(pod.Annotations, AnnotationHibernate)
		return
	}
	a := ensurePodAnnotations(pod)
	a[AnnotationHibernate] = annotationTrue
}

// ReadHibernateState reads the hibernate annotation from a pod.
func ReadHibernateState(pod *corev1.Pod) HibernateState {
	if pod == nil {
		return false
	}
	return HibernateState(pod.Annotations[AnnotationHibernate] == annotationTrue)
}
