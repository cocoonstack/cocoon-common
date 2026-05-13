package k8s

import (
	"errors"
	"fmt"
	"net"
)

// ErrNoNodeIP is returned by DetectNodeIP when no non-loopback IPv4
// address is reachable. Callers decide whether to fall back to a
// configured default; auto-substituting 127.0.0.1 would mask
// misconfigured network namespaces.
var ErrNoNodeIP = errors.New("no non-loopback IPv4 address found")

// DetectNodeIP returns the first non-loopback IPv4 address.
//
// Returns a wrapped net.InterfaceAddrs error when the host network
// stack is unavailable, or ErrNoNodeIP when every interface is
// loopback / IPv6-only.
func DetectNodeIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("list interface addresses: %w", err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}
	return "", ErrNoNodeIP
}
