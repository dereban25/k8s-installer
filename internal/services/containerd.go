package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func (m *Manager) waitForContainerd() error {
	// Таймаут можно менять через ENV
	maxRetries := 60
	if v := os.Getenv("CONTAINERD_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxRetries = n
		}
	}

	// Стартуем tail логов, чтобы видеть ошибки containerd в реальном времени
	go func() {
		cmd := exec.Command("tail", "-n", "20", "-f", "/var/log/kubernetes/containerd.log")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}()

	for i := 0; i < maxRetries; i++ {
		// Проверяем, запущен ли процесс
		if _, err := exec.Command("pgrep", "-x", "containerd").Output(); err != nil {
			log.Println("  ⚠ containerd процесс не найден")
		}

		// Проверяем, появился ли сокет
		if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
			// Пробуем crictl
			cmd := exec.Command("crictl", "--runtime-endpoint", "unix:///run/containerd/containerd.sock", "version")
			if err := cmd.Run(); err == nil {
				log.Println("  ✓ containerd готов")
				time.Sleep(2 * time.Second) // дать время на инициализацию
				return nil
			} else {
				log.Printf("  ⚠ crictl check failed: %v", err)
			}
		} else {
			log.Printf("  ⚠ containerd.sock ещё нет: %v", err)
		}

		// Прогресс лог
		if i%10 == 0 && i > 0 {
			log.Printf("  Ждём containerd... (%d/%d)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("containerd не стал готовым за %d секунд. Проверьте: tail -100 /var/log/kubernetes/containerd.log", maxRetries)
}
