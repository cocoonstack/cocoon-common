package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/cocoonstack/cocoon-common/meta"
)

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	return ctrlfake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestPatchMergeWritesDelta(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"}}
	cli := newFakeClient(t, pod.DeepCopy())

	if err := patchMerge(t.Context(), cli, pod, func(p *corev1.Pod) {
		if p.Labels == nil {
			p.Labels = map[string]string{}
		}
		p.Labels["x"] = "y"
	}); err != nil {
		t.Fatalf("patchMerge: %v", err)
	}

	var got corev1.Pod
	if err := cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "demo"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Labels["x"] != "y" {
		t.Errorf("label not persisted: %v", got.Labels)
	}
}

func TestPatchHibernateStateShortCircuitsNoOp(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
	}
	(meta.HibernateState(true)).Apply(pod)
	cli := newFakeClient(t, pod.DeepCopy())

	// A second call with the same state should be a no-op: the fake
	// client would error on Patch with an empty body, and our guard
	// prevents that.
	if err := PatchHibernateState(t.Context(), cli, pod, true); err != nil {
		t.Fatalf("no-op PatchHibernateState: %v", err)
	}
}

func TestPatchHibernateStateSetsAnnotation(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"}}
	cli := newFakeClient(t, pod.DeepCopy())

	if err := PatchHibernateState(t.Context(), cli, pod, true); err != nil {
		t.Fatalf("PatchHibernateState: %v", err)
	}

	var got corev1.Pod
	if err := cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "demo"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bool(meta.ReadHibernateState(&got)) {
		t.Errorf("hibernate annotation not persisted: %v", got.Annotations)
	}
}

func TestPatchHibernateStateClearsAnnotation(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"}}
	(meta.HibernateState(true)).Apply(pod)
	cli := newFakeClient(t, pod.DeepCopy())

	if err := PatchHibernateState(t.Context(), cli, pod, false); err != nil {
		t.Fatalf("PatchHibernateState(false): %v", err)
	}

	var got corev1.Pod
	if err := cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "demo"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, ok := got.Annotations[meta.AnnotationHibernate]; ok {
		t.Errorf("hibernate annotation should be cleared, got %v", got.Annotations)
	}
}
