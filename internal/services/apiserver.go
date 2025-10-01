package services

import (
	"crypto/tls"
	"fmt"
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
		"--v=0",
		"--cloud-provider=external",
		"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file=/tmp/sa.pub",
		"--service-account-signing-key-file=/tmp/sa.key",
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/apiserver.log"); err != nil {
		return err
	}

	// Wait for API server to be ready
	return m.waitForAPIServer()
}

func (m *Manager) waitForAPIServer() error {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(fmt.Sprintf("https://%s:6443/readyz", m.hostIP))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("API server did not become ready in time")
}