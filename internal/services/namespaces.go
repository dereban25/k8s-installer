package services

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) CreateSystemNamespaces() error {
	log.Println("  Creating system namespaces...")
	
	kubectlPath := filepath.Join(m.baseDir, "bin", "kubectl")
	
	// Wait for API to accept requests
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command(kubectlPath, "get", "--raw=/healthz")
		if err := cmd.Run(); err == nil {
			break
		}
		if i == maxRetries-1 {
			return fmt.Errorf("API server not responding after %d seconds", maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

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
			if strings.Contains(string(output), "already exists") {
				log.Printf("  Namespace '%s' already exists", ns)
			} else {
				log.Printf("  Warning: failed to create namespace '%s': %v", ns, err)
			}
		} else {
			log.Printf("  Created namespace '%s'", ns)
		}
	}

	return nil
}