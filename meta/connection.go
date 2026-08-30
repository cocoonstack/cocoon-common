package meta

import (
	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
)

// ConnectionType returns the connection protocol; a non-empty override wins over every inference.
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
