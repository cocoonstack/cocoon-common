package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

func TestEventfRecords(t *testing.T) {
	rec := record.NewFakeRecorder(1)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "ns"}}
	Eventf(rec, pod, corev1.EventTypeNormal, "Reason", "hello %s", "world")
	if got, want := <-rec.Events, "Normal Reason hello world"; got != want {
		t.Errorf("event = %q, want %q", got, want)
	}
}

func TestEventfNilRecorderIsNoop(t *testing.T) {
	Eventf(nil, &corev1.Pod{}, corev1.EventTypeNormal, "Reason", "ignored")
}
