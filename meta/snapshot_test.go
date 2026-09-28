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
