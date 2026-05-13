package k8s

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSelfSignedCertIsParseable(t *testing.T) {
	cert, err := GenerateSelfSignedCert("testhost", "10.0.0.1")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatalf("cert chain empty")
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Subject.CommonName != "testhost" {
		t.Errorf("CN = %q", parsed.Subject.CommonName)
	}
	found := false
	for _, ip := range parsed.IPAddresses {
		if ip.String() == "10.0.0.1" {
			found = true
		}
	}
	if !found {
		t.Errorf("SAN IP 10.0.0.1 missing from cert: %v", parsed.IPAddresses)
	}
}

func TestLoadOrGenerateCertFallsBackToSelfSigned(t *testing.T) {
	cert, source, err := LoadOrGenerateCert(t.Context(), "/does/not/exist.crt", "/does/not/exist.key", "host", "10.0.0.1")
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatalf("empty cert chain")
	}
	if source != "self-signed" {
		t.Errorf("source = %q, want self-signed", source)
	}
}

func TestLoadOrGenerateCertLoadsFromDisk(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")

	// Mint a fresh keypair directly so we can persist matching PEM
	// blocks — GenerateSelfSignedCert returns a tls.Certificate whose
	// private key is already in memory, so there is no second
	// roundtrip through PEM on disk to reuse.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "host"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("10.0.0.1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	_, source, err := LoadOrGenerateCert(t.Context(), certPath, keyPath, "host", "10.0.0.1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if source == "self-signed" {
		t.Errorf("expected disk source, got %q", source)
	}
}

func TestLoadOrGenerateCertExpiredDiskCertFallsBack(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	// NotAfter in the past forces the expiry branch.
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "host"},
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     time.Now().Add(-time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("10.0.0.1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	_, source, err := LoadOrGenerateCert(t.Context(), certPath, keyPath, "host", "10.0.0.1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if source != "self-signed" {
		t.Errorf("expected expired disk cert to fall back to self-signed, got source = %q", source)
	}
}

func TestDetectNodeIPReturnsSomething(t *testing.T) {
	// CI hosts are expected to have at least one non-loopback IPv4
	// interface; skip when none is present so the test reflects the
	// environment rather than masking it as a pass.
	got, err := DetectNodeIP()
	if err != nil {
		t.Skipf("no non-loopback IPv4 on this host: %v", err)
	}
	if got == "" {
		t.Errorf("DetectNodeIP returned empty string with no error")
	}
}
