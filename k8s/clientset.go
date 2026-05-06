// Package k8s provides shared Kubernetes client and helper utilities for cocoonstack projects.
package k8s

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// NewClientset returns a typed Kubernetes clientset using the rest.Config
// produced by LoadConfig (KUBECONFIG env, ~/.kube/config, or in-cluster).
// Errors are wrapped so callers can distinguish config-load failures from
// client-build failures.
func NewClientset() (kubernetes.Interface, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return cs, nil
}

// NewClientsetAndDynamic returns both a typed clientset and a dynamic
// client built from the same rest.Config, so callers that need to talk
// to CRDs alongside core resources don't call LoadConfig twice.
func NewClientsetAndDynamic() (kubernetes.Interface, dynamic.Interface, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build clientset: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return cs, dyn, nil
}
