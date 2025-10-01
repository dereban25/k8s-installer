package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

func (i *Installer) CreateDirectories() error {
	dirs := []string{
		filepath.Join(i.baseDir, "bin"),
		i.etcdDataDir,
		i.kubeletDir,
		filepath.Join(i.kubeletDir, "pki"),
		i.manifestsDir,
		"/var/log/kubernetes",
		"/etc/containerd",
		"/run/containerd",
		"/opt/cni/bin",
		i.cniConfDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if i.config.Verbose {
			fmt.Printf("  Created: %s\n", dir)
		}
	}

	return nil
}