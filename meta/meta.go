// Package meta defines shared metadata keys and naming rules used across Cocoon components.
//
// The package is split by concern: keys.go holds the annotation/label
// vocabulary, owner.go and vmname.go hold the helpers that read or build
// from that vocabulary, and connection.go derives the connection
// protocol. lifecycle.go, hibernate.go, vmspec.go, vmruntime.go, and
// pod.go layer typed contracts on top.
package meta
