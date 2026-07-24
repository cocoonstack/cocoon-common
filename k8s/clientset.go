// Package k8s provides shared Kubernetes client and helper utilities for cocoonstack projects.
package k8s

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewClientset returns a typed Kubernetes clientset built from LoadConfig.
func NewClientset() (kubernetes.Interface, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return buildClientset(cfg)
}

// NewClientsetAndDynamic returns a typed clientset and a dynamic client built from one rest.Config.
func NewClientsetAndDynamic() (kubernetes.Interface, dynamic.Interface, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	cs, err := buildClientset(cfg)
	if err != nil {
		return nil, nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return cs, dyn, nil
}

func buildClientset(cfg *rest.Config) (kubernetes.Interface, error) {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return cs, nil
}
