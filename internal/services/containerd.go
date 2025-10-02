package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// StartContainerd запускает containerd и ждёт готовности
func (m *Manager) StartContainerd() error {
	cmd := exec.Command("/opt/cni/bin/containerd", "-c", "/etc/containerd/config.toml")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	if err := m.startDaemon(cmd, "/var/log/kubernetes/containerd.log"); err != nil {
		return err
	}

	log.Println("  => Ждём запуска containerd...")
	return m.waitForContainerd()
}

// startDaemon — общий запуск демона с логированием
func (m *Manager) startDaemon(cmd *exec.Cmd, logPath string) error {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть лог %s: %w", logPath, err)
	}
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить процесс %s: %w", cmd.Path, err)
	}
	return nil
}

// waitForContainerd проверяет готовность containerd
func (m *Manager) waitForContainerd() error {
	maxRetries := 60
	if v := os.Getenv("CONTAINERD_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxRetries = n
		}
	}

	go func() {
		cmd := exec.Command("tail", "-n", "20", "-f", "/var/log/kubernetes/containerd.log")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}()

	for i := 0; i < maxRetries; i++ {
		if _, err := exec.Command("pgrep", "-x", "containerd").Output(); err != nil {
			log.Println("  ⚠ containerd процесс ещё не найден")
		}

		if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
			cmd := exec.Command("crictl", "--runtime-endpoint", "unix:///run/containerd/containerd.sock", "version")
			if err := cmd.Run(); err == nil {
				log.Println("  ✓ containerd готов")
				time.Sleep(2 * time.Second)
				return nil
			} else {
				log.Printf("  ⚠ crictl check failed: %v", err)
			}
		} else {
			log.Printf("  ⚠ containerd.sock ещё нет: %v", err)
		}

		if i%10 == 0 && i > 0 {
			log.Printf("  всё ещё ждём containerd... (%d/%d)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("containerd не стал готовым за %d секунд. tail -100 /var/log/kubernetes/containerd.log", maxRetries)
}
