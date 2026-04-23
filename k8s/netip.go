package k8s

import "net"

// DetectNodeIP returns the first non-loopback IPv4 address, or "127.0.0.1" if none found.
func DetectNodeIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return localhost
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return localhost
}
