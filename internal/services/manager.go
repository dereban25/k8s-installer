package services
import (
	"fmt"
	"os"
	"os/exec"
)
// Manager управляет системными сервисами (etcd, api-server, kubelet, containerd и т.д.)
type Manager struct {
	baseDir     string
	kubeletDir  string
	hostIP      string
	skipAPIWait bool
}

// NewManager: (string, string, string, bool) — последний флаг = skipAPIWait (fast mode)
func NewManager(baseDir, kubeletDir, hostIP string, skipAPIWait bool) *Manager {
	return &Manager{
		baseDir:     baseDir,
		kubeletDir:  kubeletDir,
		hostIP:      hostIP,
		skipAPIWait: skipAPIWait,
	}
}
// startDaemon запускает процесс и пишет stdout/stderr в лог-файл
func (m *Manager) startDaemon(cmd *exec.Cmd, logPath string) error {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть лог %s: %w", logPath, err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить %s: %w", cmd.Path, err)
	}
	return nil
}