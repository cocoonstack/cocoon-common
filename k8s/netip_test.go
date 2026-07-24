package k8s

import "testing"

func TestDetectNodeIPReturnsSomething(t *testing.T) {
	// Skip rather than fail: a host without a non-loopback IPv4 is an environment gap, not a bug.
	got, err := DetectNodeIP()
	if err != nil {
		t.Skipf("no non-loopback IPv4 on this host: %v", err)
	}
	if got == "" {
		t.Errorf("DetectNodeIP returned empty string with no error")
	}
}
