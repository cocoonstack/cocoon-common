package k8s

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultClientQPS   = 50
	defaultClientBurst = 100
)

// LoadConfig resolves a client config from $KUBECONFIG (a path list is merged, as kubectl does),
// then ~/.kube/config, then in-cluster, and applies COCOON_K8S_QPS/COCOON_K8S_BURST (default 50/100).
func LoadConfig() (*rest.Config, error) {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	cfg.QPS = float32(EnvInt("COCOON_K8S_QPS", defaultClientQPS))
	cfg.Burst = EnvInt("COCOON_K8S_BURST", defaultClientBurst)
	return cfg, nil
}
