package k8s

import "testing"

func TestDetectNodeIPReturnsSomething(t *testing.T) {
	got, err := DetectNodeIP()
	if err != nil {
		t.Skipf("no non-loopback IPv4 on this host: %v", err)
	}
	if got == "" {
		t.Errorf("DetectNodeIP returned empty string with no error")
	}
}
