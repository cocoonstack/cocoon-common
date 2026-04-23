package k8s

import (
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
)

const localhost = "127.0.0.1"

// LoadOrGenerateCert loads a TLS keypair from disk, falling back to a self-signed cert.
// Returns a source label for logging ("disk <path>" or "self-signed").
func LoadOrGenerateCert(certPath, keyPath, hostname, ip string) (tls.Certificate, string, error) {
	if certPath != "" && keyPath != "" {
		if _, err := os.Stat(certPath); err == nil {
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return tls.Certificate{}, "", fmt.Errorf("load TLS keypair %s: %w", certPath, err)
			}
			return cert, fmt.Sprintf("disk %s", certPath), nil
		}
	}
	cert, err := GenerateSelfSignedCert(hostname, ip)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("generate self-signed cert: %w", err)
	}
	return cert, "self-signed", nil
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
