package meta

import (
	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
)

// ConnectionType returns the connection protocol. A non-empty override
// wins over OS-based inference (e.g. Linux + xrdp → rdp).
func ConnectionType(osType string, hasVNCPort bool, override string) string {
	if override != "" {
		return override
	}
	switch {
	case hasVNCPort:
		return string(cocoonv1.ConnTypeVNC)
	case osType == string(cocoonv1.OSAndroid):
		return string(cocoonv1.ConnTypeADB)
	case osType == string(cocoonv1.OSWindows):
		return string(cocoonv1.ConnTypeRDP)
	default:
		return string(cocoonv1.ConnTypeSSH)
	}
}
