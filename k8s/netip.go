package k8s

import (
	"errors"
	"fmt"
	"net"
)

const cocoonBridge = "cni0"

// DetectNodeIP returns the first non-loopback IPv4 address outside the cocoon bridge.
func DetectNodeIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("list interfaces: %w", err)
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("list %s addresses: %w", iface.Name, err)
		}
		if ip, ok := nodeIPv4(iface, addrs); ok {
			return ip, nil
		}
	}
	return "", errors.New("no non-loopback IPv4 address found")
}

func nodeIPv4(iface net.Interface, addrs []net.Addr) (string, bool) {
	if iface.Flags&net.FlagLoopback != 0 || iface.Name == cocoonBridge {
		return "", false
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String(), true
		}
	}
	return "", false
}
