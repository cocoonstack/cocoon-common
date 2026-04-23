package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeKubeconfig writes a minimal valid kubeconfig under t.TempDir() and
// returns its path. The server URL is fake — good enough for the clients
// to build; no requests are issued in these tests.
func writeKubeconfig(t *testing.T) string {
	t.Helper()
	body := `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: http://127.0.0.1:1
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: test-token
`
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func TestNewClientsetFromKubeconfigEnv(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t))
	cs, err := NewClientset()
	if err != nil {
		t.Fatalf("NewClientset: %v", err)
	}
	if cs == nil {
		t.Errorf("clientset must not be nil")
	}
}

func TestNewClientsetAndDynamicFromKubeconfigEnv(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t))
	cs, dyn, err := NewClientsetAndDynamic()
	if err != nil {
		t.Fatalf("NewClientsetAndDynamic: %v", err)
	}
	if cs == nil || dyn == nil {
		t.Errorf("both clients must be non-nil, got cs=%v dyn=%v", cs, dyn)
	}
}

func TestNewClientsetBadKubeconfigWraps(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "does-not-exist"))
	_, err := NewClientset()
	if err == nil {
		t.Fatalf("expected error for missing kubeconfig")
	}
	if !strings.Contains(err.Error(), "load kubeconfig") {
		t.Errorf("want wrap prefix 'load kubeconfig', got %q", err.Error())
	}
}

func TestNewClientsetAndDynamicBadKubeconfigWraps(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "does-not-exist"))
	_, _, err := NewClientsetAndDynamic()
	if err == nil {
		t.Fatalf("expected error for missing kubeconfig")
	}
	if !strings.Contains(err.Error(), "load kubeconfig") {
		t.Errorf("want wrap prefix 'load kubeconfig', got %q", err.Error())
	}
}
