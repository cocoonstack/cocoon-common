//go:build tools

// Package tools pins build-time dependencies (controller-gen) via go.mod.
package tools

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
