package k8s

import (
	"errors"
	"fmt"
	"net"
)

const cocoonBridge = "cni0"

// DetectNodeIP returns the first IPv4 address outside loopback and the cocoon bridge.
func DetectNodeIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("list interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Name == cocoonBridge {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("list %s addresses: %w", iface.Name, err)
		}
		if ip, ok := firstIPv4(addrs); ok {
			return ip, nil
		}
	}
	return "", errors.New("no non-loopback IPv4 address found")
}

func firstIPv4(addrs []net.Addr) (string, bool) {
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String(), true
		}
	}
	return "", false
}
