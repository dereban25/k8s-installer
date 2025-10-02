package services

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
