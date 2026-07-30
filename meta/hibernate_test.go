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
	pod := &corev1.Pod{}
	// delete on a nil map must not panic.
	HibernateState(false).Apply(pod)
	if got, ok := pod.Annotations[AnnotationHibernate]; ok {
		t.Errorf("HibernateState(false) on nil annotations should remain absent, got %q", got)
	}
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

func TestMarkRestoreFromHibernate(t *testing.T) {
	pod := &corev1.Pod{}
	if ReadRestoreFromHibernate(pod) {
		t.Fatal("fresh pod should not be flagged for restore")
	}
	MarkRestoreFromHibernate(pod)
	if !ReadRestoreFromHibernate(pod) {
		t.Errorf("MarkRestoreFromHibernate should round-trip through ReadRestoreFromHibernate")
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

func TestKeepSnapshotOnDeleteRoundTrip(t *testing.T) {
	pod := &corev1.Pod{}
	if ReadKeepSnapshotOnDelete(pod) {
		t.Error("an unflagged pod must not keep its snapshot: a plain teardown has to GC it")
	}
	MarkKeepSnapshotOnDelete(pod)
	if pod.Annotations[AnnotationKeepSnapshotOnDelete] != annotationTrue {
		t.Errorf("MarkKeepSnapshotOnDelete should set %s=%s, got %q", AnnotationKeepSnapshotOnDelete, annotationTrue, pod.Annotations[AnnotationKeepSnapshotOnDelete])
	}
	if !ReadKeepSnapshotOnDelete(pod) {
		t.Error("Read must see what Mark wrote")
	}
}

func TestReadKeepSnapshotOnDeleteRejectsNonTrue(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationKeepSnapshotOnDelete: "1",
	}}}
	if ReadKeepSnapshotOnDelete(pod) {
		t.Error("only the literal \"true\" may keep a snapshot alive past its pod")
	}
}
