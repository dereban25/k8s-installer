package services

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *Manager) StartEtcd() error {
	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "etcd"),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", m.hostIP),
		"--listen-client-urls=http://0.0.0.0:2379",
		"--data-dir=./etcd",
		"--listen-peer-urls=http://0.0.0.0:2380",
		fmt.Sprintf("--initial-cluster=default=http://%s:2380", m.hostIP),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", m.hostIP),
		"--initial-cluster-state=new",
		"--initial-cluster-token=test-token",
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/etcd.log"); err != nil {
		return err
	}

	log.Println("  Waiting for etcd to become ready...")
	return m.waitForEtcd()
}

func (m *Manager) waitForEtcd() error {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(fmt.Sprintf("http://%s:2379/health", m.hostIP))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				log.Println("  ✓ Etcd is ready")
				return nil
			}
		}

		// Also try localhost
		resp, err = client.Get("http://127.0.0.1:2379/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				log.Println("  ✓ Etcd is ready")
				return nil
			}
		}

		if i%5 == 0 && i > 0 {
			log.Printf("  Still waiting for etcd... (%d/%d seconds)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("etcd did not become ready in time. Check: tail -100 /var/log/kubernetes/etcd.log")
}