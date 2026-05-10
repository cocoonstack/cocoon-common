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

func TestLifecycleStatusApply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		pod         *corev1.Pod
		status      LifecycleStatus
		wantState   string
		wantGen     string
		wantMessage string
		wantNoMsg   bool
	}{
		{
			name:        "writes triple",
			pod:         &corev1.Pod{},
			status:      LifecycleStatus{State: LifecycleStateReady, ObservedGeneration: 7, Message: "ok"},
			wantState:   "ready",
			wantGen:     "7",
			wantMessage: "ok",
		},
		{
			name:      "empty message deletes annotation",
			pod:       &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{AnnotationLifecycleStateMessage: "stale"}}},
			status:    LifecycleStatus{State: LifecycleStateReady, ObservedGeneration: 9},
			wantState: "ready",
			wantGen:   "9",
			wantNoMsg: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tt.status.Apply(tt.pod)
			if got := tt.pod.Annotations[AnnotationLifecycleState]; got != tt.wantState {
				t.Errorf("state = %q, want %q", got, tt.wantState)
			}
			if got := tt.pod.Annotations[AnnotationLifecycleObservedGeneration]; got != tt.wantGen {
				t.Errorf("generation = %q, want %q", got, tt.wantGen)
			}
			got, ok := tt.pod.Annotations[AnnotationLifecycleStateMessage]
			switch {
			case tt.wantNoMsg && ok:
				t.Errorf("message annotation should be cleared, got %q", got)
			case !tt.wantNoMsg && got != tt.wantMessage:
				t.Errorf("message = %q, want %q", got, tt.wantMessage)
			}
		})
	}
}

func TestLifecycleStatusApplyNilPod(t *testing.T) {
	t.Parallel()
	LifecycleStatus{State: LifecycleStateReady}.Apply(nil)
}

func TestLifecycleStatusPatchPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status LifecycleStatus
		want   map[string]any
	}{
		{
			"with message",
			LifecycleStatus{State: LifecycleStateFailed, ObservedGeneration: 3, Message: "boom"},
			map[string]any{
				AnnotationLifecycleState:              "failed",
				AnnotationLifecycleObservedGeneration: "3",
				AnnotationLifecycleStateMessage:       "boom",
			},
		},
		{
			"empty message deletes via nil",
			LifecycleStatus{State: LifecycleStateReady, ObservedGeneration: 1},
			map[string]any{
				AnnotationLifecycleState:              "ready",
				AnnotationLifecycleObservedGeneration: "1",
				AnnotationLifecycleStateMessage:       nil,
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.PatchPayload()
			if len(got) != len(tt.want) {
				t.Fatalf("payload size = %d, want %d", len(got), len(tt.want))
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("%s = %v, want %v", k, got[k], want)
				}
			}
		})
	}
}

func TestLifecycleStatusSnapshot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status LifecycleStatus
		want   string
	}{
		{"empty", LifecycleStatus{}, "\x000\x00"},
		{"populated", LifecycleStatus{State: LifecycleStateReady, ObservedGeneration: 7, Message: "ok"}, "ready\x007\x00ok"},
		{"message with pipe", LifecycleStatus{State: LifecycleStateFailed, Message: "a|b"}, "failed\x000\x00a|b"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Snapshot(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSnapshotDistinguishesAdjacentValues(t *testing.T) {
	t.Parallel()
	a := LifecycleStatus{State: "ab", ObservedGeneration: 0, Message: "c"}.Snapshot()
	b := LifecycleStatus{State: "a", ObservedGeneration: 0, Message: "bc"}.Snapshot()
	if a == b {
		t.Errorf("snapshots collided: %q == %q", a, b)
	}
}

func TestReadLifecycleStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pod  *corev1.Pod
		want LifecycleStatus
	}{
		{"nil pod", nil, LifecycleStatus{}},
		{"empty pod", &corev1.Pod{}, LifecycleStatus{}},
		{
			"populated",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				AnnotationLifecycleState:              "ready",
				AnnotationLifecycleObservedGeneration: "11",
				AnnotationLifecycleStateMessage:       "ok",
			}}},
			LifecycleStatus{State: LifecycleStateReady, ObservedGeneration: 11, Message: "ok"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReadLifecycleStatus(tt.pod); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
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

func TestStampCocoonSetGeneration(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{}
	StampCocoonSetGeneration(pod, 42)
	if got := pod.Annotations[AnnotationCocoonSetGeneration]; got != "42" {
		t.Errorf("annotation = %q, want %q", got, "42")
	}
	StampCocoonSetGeneration(nil, 1) // must not panic
}
