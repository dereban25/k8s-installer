package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (m *Manager) StartContainerd() error {
	// Проверяем и создаем необходимые директории для GitHub Actions
	if err := m.ensureContainerdDirectories(); err != nil {
		return fmt.Errorf("failed to create containerd directories: %v", err)
	}

	// Очищаем старые данные если они есть
	if err := m.cleanupOldContainerdData(); err != nil {
		log.Printf("Warning: failed to cleanup old data: %v", err)
	}

	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "containerd"),
		"-c", "/etc/containerd/config.toml",
		"--log-level", "info", // Добавляем явный уровень логирования
	)

	// Улучшенные переменные окружения для GitHub Actions
	cmd.Env = append(os.Environ(),
		"PATH="+os.Getenv("PATH")+":"+filepath.Join(m.baseDir, "bin")+":/usr/local/bin:/usr/sbin",
		"CONTAINERD_NAMESPACE=k8s.io", // Явно устанавливаем namespace
		"TMPDIR=/tmp",                 // Устанавливаем временную директорию
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/containerd.log"); err != nil {
		return err
	}

	log.Println("  Waiting for containerd to become ready...")
	return m.waitForContainerdWithContext()
}

func (m *Manager) ensureContainerdDirectories() error {
	dirs := []string{
		"/var/lib/containerd",
		"/run/containerd",
		"/var/log/kubernetes",
		"/tmp/containerd",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	return nil
}

func (m *Manager) cleanupOldContainerdData() error {
	// Удаляем старые сокеты и lock файлы
	filesToCleanup := []string{
		"/run/containerd/containerd.sock",
		"/run/containerd/containerd.sock.ttrpc",
		"/var/lib/containerd/io.containerd.metadata.v1.bolt/meta.db.lock",
	}

	for _, file := range filesToCleanup {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			log.Printf("  Warning: failed to remove %s: %v", file, err)
		}
	}

	return nil
}

func (m *Manager) waitForContainerdWithContext() error {
	// Увеличиваем таймаут для GitHub Actions (медленнее чем локальная среда)
	maxRetries := 120 
	if v := os.Getenv("CONTAINERD_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRetries = n
		}
	}

	// Контекст с таймаутом для общего времени ожидания
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxRetries+30)*time.Second)
	defer cancel()

	// Шаг 1: Ждем процесс containerd
	log.Println("  Waiting for containerd process...")
	if err := m.waitForContainerdProcess(ctx); err != nil {
		return err
	}

	// Шаг 2: Ждем сокет
	log.Println("  Waiting for containerd socket...")
	if err := m.waitForSocket(ctx); err != nil {
		return err
	}

	// Шаг 3: Ждем BoltDB инициализацию
	log.Println("  Waiting for metadata store...")
	if err := m.waitForMetadataStore(ctx); err != nil {
		return err
	}

	// Шаг 4: Проверяем CRI
	log.Println("  Waiting for CRI plugin...")
	return m.waitForCRIPlugin(ctx, maxRetries)
}

func (m *Manager) waitForContainerdProcess(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for containerd process")
		default:
		}

		if output, err := exec.Command("pgrep", "-f", "containerd").Output(); err == nil && len(output) > 0 {
			log.Println("  Containerd process started")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (m *Manager) waitForSocket(ctx context.Context) error {
	socketPath := "/run/containerd/containerd.sock"
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for containerd socket")
		default:
		}

		if _, err := os.Stat(socketPath); err == nil {
			// Проверяем что сокет действительно работает
			if err := m.testSocket(socketPath); err == nil {
				log.Println("  Socket created and responsive")
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (m *Manager) testSocket(socketPath string) error {
	// Простая проверка подключения к сокету
	cmd := exec.Command("timeout", "2", "ls", "-la", socketPath)
	return cmd.Run()
}

func (m *Manager) waitForMetadataStore(ctx context.Context) error {
	// Ждем пока BoltDB завершит инициализацию
	// Проверяем логи на отсутствие "waiting for response from boltdb"
	
	stableCount := 0
	requiredStableChecks := 5
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for metadata store")
		default:
		}

		// Проверяем последние строки лога
		if output, err := exec.Command("tail", "-5", "/var/log/kubernetes/containerd.log").Output(); err == nil {
			logContent := string(output)
			
			// Если нет сообщений о BoltDB проблемах в последних строках
			if !strings.Contains(logContent, "waiting for response from boltdb") &&
			   !strings.Contains(logContent, "plugin=bolt") {
				stableCount++
				if stableCount >= requiredStableChecks {
					log.Println("  Metadata store initialized")
					return nil
				}
			} else {
				stableCount = 0
			}
		}
		
		time.Sleep(1 * time.Second)
	}
}

func (m *Manager) waitForCRIPlugin(ctx context.Context, maxRetries int) error {
	criCmd := []string{
		"crictl",
		"--runtime-endpoint", "unix:///run/containerd/containerd.sock",
		"--timeout", "5s", // Добавляем таймаут для crictl
	}

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for CRI")
		default:
		}

		// Проверяем version
		versionCmd := exec.CommandContext(ctx, criCmd[0], append(criCmd[1:], "version")...)
		if err := versionCmd.Run(); err == nil {
			
			// Двойная проверка с info
			infoCmd := exec.CommandContext(ctx, criCmd[0], append(criCmd[1:], "info")...)
			if err := infoCmd.Run(); err == nil {
				log.Println("  Containerd CRI is ready")
				return nil
			}
		}

		// Показываем прогресс каждые 10 секунд
		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting for CRI... (%d/%d)", i, maxRetries)
			
			// Диагностика каждые 30 секунд
			if i%30 == 0 {
				m.showDetailedDiagnostics()
			}
		}
		
		time.Sleep(1 * time.Second)
	}

	m.showDetailedDiagnostics()
	return fmt.Errorf("containerd CRI did not respond after %d seconds", maxRetries)
}

func (m *Manager) showDetailedDiagnostics() {
	log.Println("  [Detailed CRI Diagnostics]")
	
	// Процессы
	if output, err := exec.Command("pgrep", "-a", "containerd").Output(); err == nil {
		log.Printf("  Containerd processes:\n%s", string(output))
	}
	
	// Сокеты
	if output, err := exec.Command("ls", "-la", "/run/containerd/").Output(); err == nil {
		log.Printf("  Containerd sockets:\n%s", string(output))
	}
	
	// Последние логи
	if output, err := exec.Command("tail", "-20", "/var/log/kubernetes/containerd.log").Output(); err == nil {
		log.Printf("  Last 20 log lines:\n%s", string(output))
	}
	
	// Системные ресурсы
	if output, err := exec.Command("df", "-h", "/var/lib/containerd").Output(); err == nil {
		log.Printf("  Disk space:\n%s", string(output))
	}
	
	// Память
	if output, err := exec.Command("free", "-h").Output(); err == nil {
		log.Printf("  Memory usage:\n%s", string(output))
	}

	// Попытка подключения к сокету
	if output, err := exec.Command("timeout", "5", "ctr", "--address", "/run/containerd/containerd.sock", "version").Output(); err != nil {
		log.Printf("  Direct socket test failed: %v", err)
	} else {
		log.Printf("  Direct socket test successful:\n%s", string(output))
	}
}