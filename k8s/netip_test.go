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

func TestFirstIPv4SkipsIPv6AndNonIPNetAddrs(t *testing.T) {
	addrs := []net.Addr{&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}, &net.TCPAddr{IP: net.ParseIP("10.0.0.2")}, &net.IPNet{IP: net.ParseIP("172.20.100.1"), Mask: net.CIDRMask(24, 32)}}
	if ip, ok := firstIPv4(addrs); !ok || ip != "172.20.100.1" {
		t.Fatalf("firstIPv4 = %q, %v, want the first IPv4 IPNet", ip, ok)
	}
	if ip, ok := firstIPv4(addrs[:2]); ok {
		t.Fatalf("firstIPv4 picked %q from IPv6 and non-IPNet addrs", ip)
	}
}
