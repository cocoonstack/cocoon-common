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

func TestNodeIPv4SkipsTheBridgeAndLoopback(t *testing.T) {
	addrs := []net.Addr{&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}, &net.IPNet{IP: net.ParseIP("172.20.100.1"), Mask: net.CIDRMask(24, 32)}}
	if ip, ok := nodeIPv4(net.Interface{Name: cocoonBridge, Flags: net.FlagUp}, addrs); ok {
		t.Fatalf("the cocoon bridge address %s was picked", ip)
	}
	if ip, ok := nodeIPv4(net.Interface{Name: "lo0", Flags: net.FlagUp | net.FlagLoopback}, addrs); ok {
		t.Fatalf("a loopback interface address %s was picked", ip)
	}
	if ip, ok := nodeIPv4(net.Interface{Name: "ens4", Flags: net.FlagUp}, addrs); !ok || ip != "172.20.100.1" {
		t.Fatalf("nodeIPv4(ens4) = %q, %v, want the first IPv4", ip, ok)
	}
}
