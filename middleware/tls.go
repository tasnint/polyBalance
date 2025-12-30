package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"time"
)

type TLSConfig struct {
	Enabled     bool
	CertFile    string
	KeyFile     string
	AutoGenCert bool
}

func NewTLSConfig(enabled bool, certFile, keyFile string, autoGen bool) *TLSConfig {
	return &TLSConfig{
		Enabled:     enabled,
		CertFile:    certFile,
		KeyFile:     keyFile,
		AutoGenCert: autoGen,
	}
}

func (tc *TLSConfig) GenerateSelfSignedCert() error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PolyBalance"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "*.replit.dev", "*.replit.app"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certFile, err := os.Create(tc.CertFile)
	if err != nil {
		return err
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	keyFile, err := os.Create(tc.KeyFile)
	if err != nil {
		return err
	}
	defer keyFile.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}

	return nil
}

func (tc *TLSConfig) GetTLSConfig() (*tls.Config, error) {
	if tc.AutoGenCert {
		if _, err := os.Stat(tc.CertFile); os.IsNotExist(err) {
			if err := tc.GenerateSelfSignedCert(); err != nil {
				return nil, err
			}
		}
	}

	cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}

func (tc *TLSConfig) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":       tc.Enabled,
		"cert_file":     tc.CertFile,
		"key_file":      tc.KeyFile,
		"auto_gen_cert": tc.AutoGenCert,
	}
}

func HTTPSRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") == "http" {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}
