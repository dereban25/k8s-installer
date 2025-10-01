package services

import (
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) CreateSystemNamespaces() error {
	log.Println("  Creating system namespaces...")
	
	// Wait a bit for API server to be fully ready
	time.Sleep(5 * time.Second)

	kubectlPath := filepath.Join(m.baseDir, "bin", "kubectl")

	namespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
	}

	for _, ns := range namespaces {
		cmd := exec.Command(kubectlPath, "create", "namespace", ns)
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			// Игнорируем ошибку если namespace уже существует
			if strings.Contains(string(output), "already exists") {
				log.Printf("  ✓ Namespace '%s' already exists", ns)
			} else {
				log.Printf("  ⚠ Warning: failed to create namespace '%s': %v", ns, err)
				if m.skipAPIWait {
					log.Printf("    Output: %s", string(output))
				}
			}
		} else {
			log.Printf("  ✓ Created namespace '%s'", ns)
		}
	}

	return nil
}