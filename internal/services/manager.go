package services

import (
	"log"
)

// Manager управляет системными сервисами (etcd, api-server, containerd и т.д.)
type Manager struct {
	baseDir    string
	kubeletDir string
	hostIP     string
	verbose    bool
}

// NewManager создаёт новый менеджер сервисов
func NewManager(baseDir, kubeletDir, hostIP string, verbose bool) *Manager {
	return &Manager{
		baseDir:    baseDir,
		kubeletDir: kubeletDir,
		hostIP:     hostIP,
		verbose:    verbose,
	}
}

// Лог-хелпер
func (m *Manager) logf(format string, args ...interface{}) {
	if m.verbose {
		log.Printf(format, args...)
	}
}
