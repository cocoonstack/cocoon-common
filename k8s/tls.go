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

// LoadOrGenerateCert tries to load a TLS keypair from disk first; if
// either file is missing it falls back to an in-memory self-signed
// certificate valid for hostname / ip. The self-signed path is meant
// for dev and bring-up — production installs should mount real certs
// at the supplied paths. The second return is a human-readable source
// label ("disk <path>" or "self-signed") useful for startup logs.
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

// GenerateSelfSignedCert creates an in-memory ECDSA P-256 keypair
// and returns it as a tls.Certificate. The certificate is valid for
// ten years (long enough for dev) against hostname and ip.
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
		IPAddresses:  []net.IP{net.ParseIP(ip), net.ParseIP("127.0.0.1")},
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

// DetectNodeIP walks the host's network interfaces and returns the
// first non-loopback IPv4 address. Used as the fallback when a
// component cannot discover its node IP from the configuration.
// Returns "127.0.0.1" when no interface has a usable IPv4 address.
func DetectNodeIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return "127.0.0.1"
}
