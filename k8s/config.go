package k8s

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LoadConfig returns a client config from $KUBECONFIG (an os.PathListSeparator
// list is merged, as kubectl does), then ~/.kube/config, then in-cluster — the
// deferred loading rules cover all three, first match wins.
func LoadConfig() (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}
