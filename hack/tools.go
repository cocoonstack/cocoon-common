//go:build tools

// Package tools pins build-time dependencies (controller-gen) via go.mod
// so that the version is reproducible across contributor machines and CI.
//
// The build tag prevents this file from being included in normal builds —
// the imports exist solely to keep their modules in go.sum.
package tools

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
