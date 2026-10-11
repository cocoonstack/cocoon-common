package meta

import (
	"maps"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReadSnapshotAnnotations(t *testing.T) {
	tests := []struct {
		name    string
		ann     map[string]string
		want    map[string]string
		wantErr bool
	}{
		{name: "absent", ann: nil, want: nil},
		{name: "empty", ann: map[string]string{AnnotationSnapshotAnnotations: ""}, want: nil},
		{
			name: "object",
			ann:  map[string]string{AnnotationSnapshotAnnotations: `{"ai.simular.owner":"org:osworld","ai.simular.manifest":"sha256:ab"}`},
			want: map[string]string{"ai.simular.owner": "org:osworld", "ai.simular.manifest": "sha256:ab"},
		},
		{name: "malformed", ann: map[string]string{AnnotationSnapshotAnnotations: `{"a":`}, wantErr: true},
		{name: "non-string value", ann: map[string]string{AnnotationSnapshotAnnotations: `{"a":1}`}, wantErr: true},
		{name: "reserved key", ann: map[string]string{AnnotationSnapshotAnnotations: `{"cocoonstack.snapshot.baseimage":"x"}`}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: tt.ann}}
			got, err := ReadSnapshotAnnotations(pod)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnapshotStatusRoundTrip(t *testing.T) {
	want := SnapshotStatus{Ref: "user-snapshots/u1/s1", State: SnapshotStateFailed, Message: "vm is not running"}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
	for k, v := range want.Annotations() {
		pod.Annotations[k] = v.(string)
	}
	got, ok := ReadSnapshotStatus(pod)
	if !ok || got != want {
		t.Fatalf("got %+v ok=%v, want %+v", got, ok, want)
	}
}

func TestReadSnapshotStatusUnanswered(t *testing.T) {
	for name, raw := range map[string]string{"absent": "", "malformed": `{"ref":`, "no ref": `{"state":"pushed"}`} {
		t.Run(name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{AnnotationSnapshotStatus: raw}}}
			if _, ok := ReadSnapshotStatus(pod); ok {
				t.Error("want unanswered")
			}
		})
	}
}
