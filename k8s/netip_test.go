package k8s

import (
	"net"
	"testing"
)

func TestDetectNodeIPReturnsRoutableIPv4(t *testing.T) {
	got, err := DetectNodeIP()
	if err != nil {
		t.Skipf("no non-loopback IPv4 on this host: %v", err)
	}
	ip := net.ParseIP(got)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() {
		t.Errorf("DetectNodeIP returned %q, want a non-loopback IPv4", got)
	}
}
