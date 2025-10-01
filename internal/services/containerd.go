package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func (m *Manager) StartContainerd() error {
	cmd := exec.Command("/opt/cni/bin/containerd", "-c", "/etc/containerd/config.toml")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	if err := m.startDaemon(cmd, "/var/log/kubernetes/containerd.log"); err != nil {
		return err
	}

	log.Println("  Waiting for containerd to become ready...")
	return m.waitForContainerd()
}

func (m *Manager) waitForContainerd() error {
	maxRetries := 60
	for i := 0; i < maxRetries; i++ {
		// Check if containerd socket exists
		if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
			// Try to connect using crictl
			cmd := exec.Command("crictl", "--runtime-endpoint", "unix:///run/containerd/containerd.sock", "version")
			if err := cmd.Run(); err == nil {
				log.Println("  ✓ Containerd is ready")
				// Give it extra time to fully initialize
				time.Sleep(3 * time.Second)
				return nil
			}
		}

		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting for containerd... (%d/%d seconds)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("containerd did not become ready in %d seconds. Check: tail -100 /var/log/kubernetes/containerd.log", maxRetries)
}