package services

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *Manager) StartAPIServer() error {
	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "kube-apiserver"),
		fmt.Sprintf("--etcd-servers=http://%s:2379", m.hostIP),
		"--service-cluster-ip-range=10.0.0.0/24",
		"--bind-address=0.0.0.0",
		"--secure-port=6443",
		fmt.Sprintf("--advertise-address=%s", m.hostIP),
		"--authorization-mode=AlwaysAllow",
		"--token-auth-file=/tmp/token.csv",
		"--enable-priority-and-fairness=false",
		"--allow-privileged=true",
		"--profiling=false",
		"--storage-backend=etcd3",
		"--storage-media-type=application/json",
		"--v=2",
		"--cloud-provider=external",
		"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file=/tmp/sa.pub",
		"--service-account-signing-key-file=/tmp/sa.key",
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/apiserver.log"); err != nil {
		return err
	}

	log.Println("  Waiting for API server to become ready...")
	// Wait for API server to be ready
	return m.waitForAPIServer()
}

func (m *Manager) waitForAPIServer() error {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	maxRetries := 60 // Увеличено до 60 попыток (2 минуты)
	for i := 0; i < maxRetries; i++ {
		// Проверяем по localhost и по hostIP
		urls := []string{
			"https://127.0.0.1:6443/readyz",
			fmt.Sprintf("https://%s:6443/readyz", m.hostIP),
		}

		for _, url := range urls {
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					log.Printf("  ✓ API server is ready (attempt %d/%d)", i+1, maxRetries)
					return nil
				}
			}
		}

		if i%5 == 0 && i > 0 {
			log.Printf("  Still waiting for API server... (%d/%d)", i, maxRetries)
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("API server did not become ready after %d seconds. Check logs at /var/log/kubernetes/apiserver.log", maxRetries*2)
}