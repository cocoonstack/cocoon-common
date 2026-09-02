package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPatchWritesDelta(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"}}
	cli := newFakeClient(t, pod.DeepCopy())

	if err := Patch(t.Context(), cli, pod, func(p *corev1.Pod) {
		if p.Labels == nil {
			p.Labels = map[string]string{}
		}
		p.Labels["x"] = "y"
	}); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	var got corev1.Pod
	if err := cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "demo"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Labels["x"] != "y" {
		t.Errorf("label not persisted: %v", got.Labels)
	}
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	return ctrlfake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}
