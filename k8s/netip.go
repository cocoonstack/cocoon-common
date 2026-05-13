package k8s

import (
	"errors"
	"fmt"
	"net"
)

// ErrNoNodeIP is returned when no non-loopback IPv4 address is
// reachable. Callers pick the fallback — auto-substituting localhost
// would mask misconfigured network namespaces.
var ErrNoNodeIP = errors.New("no non-loopback IPv4 address found")

// DetectNodeIP returns the first non-loopback IPv4 address, or
// ErrNoNodeIP if none exists.
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
