package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLifecycleStateIsTerminal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state LifecycleState
		want  bool
	}{
		{LifecycleStateCreating, false},
		{LifecycleStateHibernating, false},
		{LifecycleStateReady, true},
		{LifecycleStateHibernated, true},
		{LifecycleStateFailed, true},
		{LifecycleState(""), false},
		{LifecycleState("unknown"), false},
	}
	for _, tt := range cases {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.want {
				t.Errorf("IsTerminal(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestReadLifecycleState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pod  *corev1.Pod
		want LifecycleState
	}{
		{"nil pod", nil, ""},
		{"missing annotation", &corev1.Pod{}, ""},
		{
			"set",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationLifecycleState: "ready"},
			}},
			LifecycleStateReady,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReadLifecycleState(tt.pod); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadLifecycleObservedGeneration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pod  *corev1.Pod
		want int64
	}{
		{"nil pod", nil, 0},
		{"missing", &corev1.Pod{}, 0},
		{
			"valid",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationLifecycleObservedGeneration: "42"},
			}},
			42,
		},
		{
			"unparseable falls back to zero",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationLifecycleObservedGeneration: "abc"},
			}},
			0,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReadLifecycleObservedGeneration(tt.pod); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReadCocoonSetGeneration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pod  *corev1.Pod
		want int64
	}{
		{"nil pod", nil, 0},
		{"missing", &corev1.Pod{}, 0},
		{
			"valid",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationCocoonSetGeneration: "7"},
			}},
			7,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReadCocoonSetGeneration(tt.pod); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
