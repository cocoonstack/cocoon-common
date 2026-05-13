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
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"time"

	"github.com/projecteru2/core/log"
)

const localhost = "127.0.0.1"

// LoadOrGenerateCert loads a TLS keypair from disk, falling back to a
// self-signed cert when paths are empty, the cert is missing, or the
// cert is expired. Returns a source label for logging.
func LoadOrGenerateCert(ctx context.Context, certPath, keyPath, hostname, ip string) (tls.Certificate, string, error) {
	cert, source, err := tryLoadDiskCert(ctx, certPath, keyPath)
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

// tryLoadDiskCert returns ("", "", nil) when the caller should fall
// back to self-signed (paths empty, cert missing, or expired) and an
// error only when a configured keypair fails to load.
func tryLoadDiskCert(ctx context.Context, certPath, keyPath string) (tls.Certificate, string, error) {
	if certPath == "" || keyPath == "" {
		return tls.Certificate{}, "", nil
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return tls.Certificate{}, "", nil
		}
		return tls.Certificate{}, "", fmt.Errorf("load tls keypair %s: %w", certPath, err)
	}
	if isCertExpired(ctx, cert, certPath) {
		return tls.Certificate{}, "", nil
	}
	return cert, fmt.Sprintf("disk %s", certPath), nil
}

// isCertExpired returns true when the leaf cert is past NotAfter.
// Parse failures are warned and treated as "not expired".
func isCertExpired(ctx context.Context, cert tls.Certificate, certPath string) bool {
	logger := log.WithFunc("k8s.LoadOrGenerateCert")
	if len(cert.Certificate) == 0 {
		return false
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		logger.Warnf(ctx, "parse disk cert %s: %v (keeping cert)", certPath, err)
		return false
	}
	if time.Now().After(parsed.NotAfter) {
		logger.Warnf(ctx, "disk cert %s expired at %s, falling back to self-signed", certPath, parsed.NotAfter.Format(time.RFC3339))
		return true
	}
	return false
}

// GenerateSelfSignedCert creates an in-memory ECDSA P-256 self-signed
// cert for hostname and ip.
func GenerateSelfSignedCert(hostname, ip string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: hostname},
		NotBefore:    now,
		NotAfter:     now.Add(10 * 365 * 24 * time.Hour),
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
