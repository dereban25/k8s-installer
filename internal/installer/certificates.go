package installer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func (i *Installer) GenerateCertificates() error {
	pkiDir := filepath.Join(i.baseDir, "pki")
	
	if _, err := os.Stat(pkiDir); os.IsNotExist(err) {
		return fmt.Errorf("PKI directory does not exist: %s (run CreateDirectories first)", pkiDir)
	}

	log.Println("  Generating CA certificate...")
	caKey, caCert, err := i.generateCA()
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	if err := i.saveCertificate(filepath.Join(pkiDir, "ca.crt"), caCert); err != nil {
		return err
	}
	if err := i.savePrivateKey(filepath.Join(pkiDir, "ca.key"), caKey); err != nil {
		return err
	}

	log.Println("  Generating admin certificate...")
	adminKey, adminCert, err := i.generateClientCert(caKey, caCert, "admin", "system:masters")
	if err != nil {
		return fmt.Errorf("failed to generate admin cert: %w", err)
	}

	if err := i.saveCertificate(filepath.Join(pkiDir, "admin.crt"), adminCert); err != nil {
		return err
	}
	if err := i.savePrivateKey(filepath.Join(pkiDir, "admin.key"), adminKey); err != nil {
		return err
	}

	log.Println("  Generating API server certificate...")
	apiKey, apiCert, err := i.generateAPIServerCert(caKey, caCert)
	if err != nil {
		return fmt.Errorf("failed to generate API server cert: %w", err)
	}

	if err := i.saveCertificate(filepath.Join(pkiDir, "apiserver.crt"), apiCert); err != nil {
		return err
	}
	if err := i.savePrivateKey(filepath.Join(pkiDir, "apiserver.key"), apiKey); err != nil {
		return err
	}

	log.Println("  Generating service account keys...")
	saKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate service account key: %w", err)
	}

	if err := i.savePrivateKey(filepath.Join(pkiDir, "sa.key"), saKey); err != nil {
		return err
	}
	if err := i.savePublicKey(filepath.Join(pkiDir, "sa.pub"), &saKey.PublicKey); err != nil {
		return err
	}

	kubeletPkiDir := filepath.Join(i.kubeletDir, "pki")
	if err := i.copyCertificate(
		filepath.Join(pkiDir, "ca.crt"),
		filepath.Join(kubeletPkiDir, "ca.crt"),
	); err != nil {
		log.Printf("Warning: failed to copy CA to kubelet dir: %v", err)
	}

	if err := i.copyCertificate(
		filepath.Join(pkiDir, "ca.crt"),
		filepath.Join(i.kubeletDir, "ca.crt"),
	); err != nil {
		log.Printf("Warning: failed to copy CA to kubelet root: %v", err)
	}

	log.Println("  All certificates generated successfully")
	return nil
}

func (i *Installer) generateCA() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "kubernetes-ca",
			Organization: []string{"Kubernetes"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

func (i *Installer) generateClientCert(caKey *rsa.PrivateKey, caCert *x509.Certificate, cn, org string) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{org},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

func (i *Installer) generateAPIServerCert(caKey *rsa.PrivateKey, caCert *x509.Certificate) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	hostIP := os.Getenv("K8S_HOST_IP")
	if hostIP == "" {
		hostIP = "127.0.0.1"
	}

	ipAddresses := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("10.0.0.1"),
	}
	
	if parsedIP := net.ParseIP(hostIP); parsedIP != nil && !parsedIP.IsLoopback() {
		ipAddresses = append(ipAddresses, parsedIP)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: "kube-apiserver",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		IPAddresses: ipAddresses,
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			"localhost",
		},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

func (i *Installer) saveCertificate(path string, cert *x509.Certificate) error {
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
	return os.WriteFile(path, certPEM, 0644)
}

func (i *Installer) savePrivateKey(path string, key *rsa.PrivateKey) error {
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return os.WriteFile(path, keyPEM, 0600)
}

func (i *Installer) savePublicKey(path string, key *rsa.PublicKey) error {
	pubBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	return os.WriteFile(path, pubPEM, 0644)
}

func (i *Installer) copyCertificate(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}