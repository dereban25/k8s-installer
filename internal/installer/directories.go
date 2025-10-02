package installer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// CreateDirectories создает все необходимые директории для установки
func (i *Installer) CreateDirectories() error {
	dirs := []string{
		// Основные директории Kubernetes
		i.baseDir,
		filepath.Join(i.baseDir, "bin"),
		filepath.Join(i.baseDir, "pki"),        // 🔑 КРИТИЧНО: директория для сертификатов
		i.etcdDataDir,
		i.manifestsDir,
		
		// Директории для kubelet
		i.kubeletDir,
		filepath.Join(i.kubeletDir, "pki"),     // 🔑 КРИТИЧНО: сертификаты kubelet
		
		// Директории для логов
		"/var/log/kubernetes",
		
		// Директории для containerd
		"/etc/containerd",
		"/run/containerd",
		
		// Директории для CNI
		i.cniConfDir,
		"/opt/cni/bin",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		log.Printf("  Created: %s", dir)
	}

	return nil
}