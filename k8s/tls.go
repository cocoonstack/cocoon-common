package k8s

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/projecteru2/core/log"
)

const localhost = "127.0.0.1"

// LoadOrGenerateCert loads a TLS keypair from disk, falling back to a self-signed cert.
// Returns a source label for logging ("disk <path>" or "self-signed").
//
// If the on-disk cert is already expired, a warning is logged and the
// function falls through to mint a self-signed cert — kubelets that
// keep running with a stale cert appear healthy but get rejected by the
// API server with an opaque TLS error.
func LoadOrGenerateCert(certPath, keyPath, hostname, ip string) (tls.Certificate, string, error) {
	cert, source, err := tryLoadDiskCert(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	if source != "" {
		return cert, source, nil
	}
	cert, err = GenerateSelfSignedCert(hostname, ip)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("generate self-signed cert: %w", err)
	}
	return cert, "self-signed", nil
}

// tryLoadDiskCert attempts to load the keypair at the configured paths.
// Returns ("", "", nil) when the caller should fall back to a self-signed
// cert (paths empty, cert missing, or cert expired) and propagates a
// non-nil error only when the keypair load itself fails — that signals
// operator misconfiguration which silently substituting a self-signed
// cert would mask.
func tryLoadDiskCert(certPath, keyPath string) (tls.Certificate, string, error) {
	if certPath == "" || keyPath == "" {
		return tls.Certificate{}, "", nil
	}
	if _, err := os.Stat(certPath); err != nil {
		return tls.Certificate{}, "", nil
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("load tls keypair %s: %w", certPath, err)
	}
	if isCertExpired(cert, certPath) {
		return tls.Certificate{}, "", nil
	}
	return cert, fmt.Sprintf("disk %s", certPath), nil
}

// isCertExpired returns true when the on-disk leaf cert is past its
// NotAfter. Parse failures are logged as warnings (not fatal) and
// treated as "not expired" so a load that succeeded at the tls layer
// is not gratuitously discarded.
func isCertExpired(cert tls.Certificate, certPath string) bool {
	logger := log.WithFunc("k8s.LoadOrGenerateCert")
	if len(cert.Certificate) == 0 {
		return false
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		logger.Warnf(context.Background(), "parse disk cert %s: %v (keeping cert)", certPath, err)
		return false
	}
	if time.Now().After(parsed.NotAfter) {
		logger.Warnf(context.Background(), "disk cert %s expired at %s, falling back to self-signed", certPath, parsed.NotAfter.Format(time.RFC3339))
		return true
	}
	return false
}

// GenerateSelfSignedCert creates an in-memory ECDSA P-256 self-signed cert for hostname and ip.
func GenerateSelfSignedCert(hostname, ip string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: hostname},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{hostname, "localhost"},
		IPAddresses:  []net.IP{net.ParseIP(ip), net.ParseIP(localhost)},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	return tls.X509KeyPair(certPEM, keyPEM)
}
