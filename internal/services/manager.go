package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/k8s-installer/internal/utils"
)

type Manager struct {
	baseDir    string
	kubeletDir string
	hostIP     string
}

func NewManager(baseDir, kubeletDir, hostIP string) *Manager {
	return &Manager{
		baseDir:    baseDir,
		kubeletDir: kubeletDir,
		hostIP:     hostIP,
	}
}

func (m *Manager) startDaemon(cmd *exec.Cmd, logFile string) error {
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		logF.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for the process to finish (it's a daemon)
	go func() {
		cmd.Wait()
		logF.Close()
	}()

	return nil
}