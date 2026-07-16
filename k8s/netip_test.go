package k8s

import "testing"

func TestDetectNodeIPReturnsSomething(t *testing.T) {
	// CI hosts are expected to have at least one non-loopback IPv4
	// interface; skip when none is present so the test reflects the
	// environment rather than masking it as a pass.
	got, err := DetectNodeIP()
	if err != nil {
		t.Skipf("no non-loopback IPv4 on this host: %v", err)
	}
	if got == "" {
		t.Errorf("DetectNodeIP returned empty string with no error")
	}
}
