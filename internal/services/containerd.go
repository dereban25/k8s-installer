package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func (m *Manager) StartContainerd() error {
	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "containerd"),
		"-c", "/etc/containerd/config.toml",
	)

	cmd.Env = append(os.Environ(),
		"PATH="+os.Getenv("PATH")+":"+filepath.Join(m.baseDir, "bin")+":/usr/local/bin:/usr/sbin",
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/containerd.log"); err != nil {
		return err
	}

	log.Println("  Waiting for containerd CRI to become ready...")
	return m.waitForContainerd()
}

func (m *Manager) waitForContainerd() error {
	maxRetries := 90 // Увеличено для CI
	if v := os.Getenv("CONTAINERD_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRetries = n
		}
	}

	// Шаг 1: Ждем сокет
	log.Println("  Waiting for containerd socket...")
	socketFound := false
	for i := 0; i < 30; i++ {
		if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
			socketFound = true
			log.Println("  Socket created")
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !socketFound {
		return fmt.Errorf("containerd socket not created after 30 seconds")
	}

	// Шаг 2: ТОЛЬКО CRI проверка - никаких fallback
	log.Println("  Waiting for CRI plugin...")
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("crictl",
			"--runtime-endpoint", "unix:///run/containerd/containerd.sock",
			"version")
		
		if err := cmd.Run(); err == nil {
			// Двойная проверка
			infoCmd := exec.Command("crictl",
				"--runtime-endpoint", "unix:///run/containerd/containerd.sock",
				"info")
			if err := infoCmd.Run(); err == nil {
				log.Println("  Containerd CRI is ready")
				return nil
			}
		}

		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting for CRI... (%d/%d)", i, maxRetries)
			if i%20 == 0 {
				m.showCRIDiagnostics()
			}
		}
		time.Sleep(1 * time.Second)
	}

	m.showCRIDiagnostics()
	return fmt.Errorf("containerd CRI did not respond after %d seconds. Check: tail -100 /var/log/kubernetes/containerd.log", maxRetries)
}

func (m *Manager) showCRIDiagnostics() {
	log.Println("  [CRI Diagnostics]")
	
	if output, err := exec.Command("pgrep", "-a", "containerd").Output(); err == nil {
		log.Printf("  Process: %s", string(output))
	}
	
	if output, err := exec.Command("tail", "-10", "/var/log/kubernetes/containerd.log").Output(); err == nil {
		log.Printf("  Last 10 log lines:\n%s", string(output))
	}
}