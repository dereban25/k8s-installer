package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (m *Manager) StartScheduler() error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/root"
	}

	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "kube-scheduler"),
		fmt.Sprintf("--kubeconfig=%s/.kube/config", homeDir),
		"--leader-elect=false",
		"--v=2",
		"--bind-address=0.0.0.0",
	)

	return m.startDaemon(cmd, "/var/log/kubernetes/scheduler.log")
}