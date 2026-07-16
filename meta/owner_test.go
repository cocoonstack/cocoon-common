package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOwnerDeploymentName(t *testing.T) {
	cases := []struct {
		name   string
		owners []metav1.OwnerReference
		want   string
		wantOK bool
	}{
		{
			name:   "replicaset with hash suffix",
			owners: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "demo-7b7c9d9d5f"}},
			want:   "demo",
			wantOK: true,
		},
		{
			name:   "no owners",
			owners: nil,
			wantOK: false,
		},
		{
			name:   "non-replicaset owner",
			owners: []metav1.OwnerReference{{Kind: "Deployment", Name: "demo"}},
			wantOK: false,
		},
		{
			name:   "replicaset with no hash suffix",
			owners: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "demo"}},
			wantOK: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := OwnerDeploymentName(tt.owners)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasCocoonTolerationKey(t *testing.T) {
	tolerations := []corev1.Toleration{{Key: TolerationKey}}
	if !HasCocoonTolerationKey(tolerations) {
		t.Fatalf("expected toleration to be detected")
	}
	if HasCocoonTolerationKey(nil) {
		t.Errorf("expected nil tolerations to be rejected")
	}
	if HasCocoonTolerationKey([]corev1.Toleration{{Key: "other"}}) {
		t.Errorf("expected unrelated toleration to be rejected")
	}
}
