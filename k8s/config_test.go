package k8s

import "testing"

func TestLoadConfigDefaultsClientRateLimits(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t))
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.QPS != defaultClientQPS || cfg.Burst != defaultClientBurst {
		t.Errorf("QPS/Burst = %v/%d, want %d/%d", cfg.QPS, cfg.Burst, defaultClientQPS, defaultClientBurst)
	}
}

func TestLoadConfigRateLimitEnvOverrides(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t))
	t.Setenv("COCOON_K8S_QPS", "7")
	t.Setenv("COCOON_K8S_BURST", "9")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.QPS != 7 || cfg.Burst != 9 {
		t.Errorf("QPS/Burst = %v/%d, want env overrides 7/9", cfg.QPS, cfg.Burst)
	}
}
