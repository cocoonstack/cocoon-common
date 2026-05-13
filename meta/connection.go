package meta

import (
	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
)

// ConnectionType returns the connection protocol. A non-empty override
// (typically AnnotationConnType) wins over OS-based inference, so a Linux
// image running xrdp can advertise rdp without faking its OS field.
func ConnectionType(osType string, hasVNCPort bool, override string) string {
	if override != "" {
		return override
	}
	switch {
	case hasVNCPort:
		return string(cocoonv1.ConnTypeVNC)
	case osType == "android":
		return string(cocoonv1.ConnTypeADB)
	case osType == "windows":
		return string(cocoonv1.ConnTypeRDP)
	default:
		return string(cocoonv1.ConnTypeSSH)
	}
}
