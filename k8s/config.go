package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LoadConfig returns a Kubernetes client config picked from the first
// matching source — there is no fallback if a chosen source errors:
//
//  1. $KUBECONFIG, if set (returns whatever BuildConfigFromFlags reports,
//     including errors when the path is missing or malformed).
//  2. ~/.kube/config, if it exists.
//  3. in-cluster service account config.
func LoadConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if home, err := os.UserHomeDir(); err == nil {
		kubeconfig := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(kubeconfig); err == nil {
			return clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}
	return rest.InClusterConfig()
}
