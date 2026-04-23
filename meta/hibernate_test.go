package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHibernateStateApplyTrue(t *testing.T) {
	pod := &corev1.Pod{}
	HibernateState(true).Apply(pod)
	if pod.Annotations[AnnotationHibernate] != annotationTrue {
		t.Errorf("HibernateState(true) should set %s=%s", AnnotationHibernate, annotationTrue)
	}
}

func TestHibernateStateApplyFalseOnNilAnnotations(t *testing.T) {
	pod := &corev1.Pod{} // pod.Annotations is nil
	// delete on a nil map must not panic.
	HibernateState(false).Apply(pod)
	if got, ok := pod.Annotations[AnnotationHibernate]; ok {
		t.Errorf("HibernateState(false) on nil annotations should remain absent, got %q", got)
	}
}

func TestHibernateStateApplyFalseNilPod(t *testing.T) {
	HibernateState(false).Apply(nil) // must not panic
}

func TestHibernateStateApplyFalseRemoves(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationHibernate: "true",
	}}}
	HibernateState(false).Apply(pod)
	if _, ok := pod.Annotations[AnnotationHibernate]; ok {
		t.Errorf("HibernateState(false) should delete the annotation, not write false")
	}
}

func TestReadHibernateState(t *testing.T) {
	cases := []struct {
		name string
		ann  map[string]string
		want HibernateState
	}{
		{"missing", nil, false},
		{"true", map[string]string{AnnotationHibernate: "true"}, true},
		{"false-string", map[string]string{AnnotationHibernate: "false"}, false},
		{"empty", map[string]string{AnnotationHibernate: ""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: c.ann}}
			if got := ReadHibernateState(pod); got != c.want {
				t.Errorf("ReadHibernateState = %v, want %v", got, c.want)
			}
		})
	}
}

func TestReadHibernateStateNilPod(t *testing.T) {
	if got := ReadHibernateState(nil); got {
		t.Errorf("ReadHibernateState(nil) = true, want false")
	}
}

func TestHibernateSnapshotTagConstant(t *testing.T) {
	if HibernateSnapshotTag != "hibernate" {
		t.Errorf("HibernateSnapshotTag = %q, want %q", HibernateSnapshotTag, "hibernate")
	}
}

func TestDefaultSnapshotTagConstant(t *testing.T) {
	if DefaultSnapshotTag != "latest" {
		t.Errorf("DefaultSnapshotTag = %q, want %q", DefaultSnapshotTag, "latest")
	}
	if DefaultSnapshotTag == HibernateSnapshotTag {
		t.Errorf("DefaultSnapshotTag must differ from HibernateSnapshotTag")
	}
}
