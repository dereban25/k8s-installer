package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
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
	// Таймаут можно задать через env: CONTAINERD_MAX_RETRIES (в секундах)
	maxRetries := 60
	if v := os.Getenv("CONTAINERD_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRetries = n
		}
	}

	// Одновременный tail логов — сразу видны реальные причины падения
	go func() {
		cmd := exec.Command("tail", "-n", "20", "-f", "/var/log/kubernetes/containerd.log")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}()

	for i := 0; i < maxRetries; i++ {
		// Есть ли процесс?
		if _, err := exec.Command("pgrep", "-x", "containerd").Output(); err != nil && i%5 == 0 {
			log.Println("  ⚠ containerd process not found yet")
		}

		// Появился ли сокет?
		if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
			// Проверяем доступ через crictl
			cmd := exec.Command("crictl", "--runtime-endpoint", "unix:///run/containerd/containerd.sock", "version")
			if err := cmd.Run(); err == nil {
				log.Println("  ✓ Containerd is ready")
				time.Sleep(2 * time.Second) // дать инициализироваться
				return nil
			} else if i%5 == 0 {
				log.Printf("  ⚠ crictl check failed: %v", err)
			}
		} else if i%5 == 0 {
			log.Printf("  ⚠ containerd.sock not found yet: %v", err)
		}

		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting for containerd... (%d/%d seconds)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("containerd did not become ready in %d seconds. Check: tail -100 /var/log/kubernetes/containerd.log", maxRetries)
}
