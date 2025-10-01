package installer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func (i *Installer) GenerateCertificates() error {
	if err := i.generateCA(); err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	if err := i.generateServiceAccountKeys(); err != nil {
		return fmt.Errorf("failed to generate service account keys: %w", err)
	}

	if err := i.createTokenFile(); err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}

	return nil
}

func (i *Installer) generateCA() error {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "kubelet-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// Save CA certificate
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})
	
	certPaths := []string{
		"/tmp/ca.crt",
		filepath.Join(i.kubeletDir, "ca.crt"),
		filepath.Join(i.kubeletDir, "pki", "ca.crt"),
	}

	for _, path := range certPaths {
		if err := os.WriteFile(path, caCertPEM, 0644); err != nil {
			return fmt.Errorf("failed to write CA certificate to %s: %w", path, err)
		}
	}

	// Save CA key
	caKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})
	if err := os.WriteFile("/tmp/ca.key", caKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	return nil
}

func (i *Installer) generateServiceAccountKeys() error {
	saKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate SA key: %w", err)
	}

	// Save private key
	saKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(saKey),
	})
	if err := os.WriteFile("/tmp/sa.key", saKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write SA key: %w", err)
	}

	// Save public key
	saPubPEM, err := x509.MarshalPKIXPublicKey(&saKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal SA public key: %w", err)
	}

	saPubKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: saPubPEM})
	if err := os.WriteFile("/tmp/sa.pub", saPubKeyPEM, 0644); err != nil {
		return fmt.Errorf("failed to write SA public key: %w", err)
	}

	return nil
}

func (i *Installer) createTokenFile() error {
	tokenContent := "1234567890,admin,admin,system:masters\n"
	if err := os.WriteFile("/tmp/token.csv", []byte(tokenContent), 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}
	return nil
}