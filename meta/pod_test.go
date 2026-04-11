package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPodReady(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{"nil", nil, false},
		{"no conditions", &corev1.Pod{}, false},
		{"ready true", &corev1.Pod{Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		}}, true},
		{"ready false", &corev1.Pod{Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		}}, false},
		{"other condition true", &corev1.Pod{Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodInitialized, Status: corev1.ConditionTrue}},
		}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsPodReady(c.pod); got != c.want {
				t.Errorf("IsPodReady = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsPodTerminal(t *testing.T) {
	if IsPodTerminal(nil) {
		t.Errorf("nil pod must not be terminal")
	}
	if !IsPodTerminal(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}}) {
		t.Errorf("PodFailed should be terminal")
	}
	if IsPodTerminal(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}) {
		t.Errorf("PodSucceeded must not be terminal — managed pods are long-running")
	}
	if IsPodTerminal(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}) {
		t.Errorf("PodRunning must not be terminal")
	}
}

func TestIsContainerRunning(t *testing.T) {
	if IsContainerRunning(nil) {
		t.Errorf("nil pod must not be running")
	}
	running := &corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		}},
	}}
	if !IsContainerRunning(running) {
		t.Errorf("running container should be detected")
	}
	waiting := &corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
		}},
	}}
	if IsContainerRunning(waiting) {
		t.Errorf("waiting container must not be reported as running")
	}
}

func TestPodKey(t *testing.T) {
	if got := PodKey("ns", "demo"); got != "ns/demo" {
		t.Errorf("PodKey = %q, want ns/demo", got)
	}
}

func TestPodNodePool(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want string
	}{
		{"nil", nil, DefaultNodePool},
		{"none set", &corev1.Pod{}, DefaultNodePool},
		{"nodeSelector wins", &corev1.Pod{
			Spec: corev1.PodSpec{NodeSelector: map[string]string{LabelNodePool: "sel"}},
			ObjectMeta: metav1.ObjectMeta{
				Labels:      map[string]string{LabelNodePool: "lab"},
				Annotations: map[string]string{LabelNodePool: "ann"},
			},
		}, "sel"},
		{"labels over annotations", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{LabelNodePool: "lab"},
			Annotations: map[string]string{LabelNodePool: "ann"},
		}}, "lab"},
		{"annotation fallback", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{LabelNodePool: "ann"},
		}}, "ann"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PodNodePool(c.pod); got != c.want {
				t.Errorf("PodNodePool = %q, want %q", got, c.want)
			}
		})
	}
}
