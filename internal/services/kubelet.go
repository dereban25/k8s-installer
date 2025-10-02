package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) StartKubelet() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "kubelet"),
		fmt.Sprintf("--kubeconfig=%s/kubeconfig", m.kubeletDir),
		fmt.Sprintf("--config=%s/config.yaml", m.kubeletDir),
		fmt.Sprintf("--root-dir=%s", m.kubeletDir),
		fmt.Sprintf("--cert-dir=%s/pki", m.kubeletDir),
		fmt.Sprintf("--hostname-override=%s", hostname),
		"--pod-infra-container-image=registry.k8s.io/pause:3.10",
		fmt.Sprintf("--node-ip=%s", m.hostIP),
		"--cloud-provider=external",
		"--cgroup-driver=cgroupfs",
		"--max-pods=10",
		"--runtime-request-timeout=5m",
		"--v=1",
	)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	if err := m.startDaemon(cmd, "/var/log/kubernetes/kubelet.log"); err != nil {
		return err
	}

	log.Println("  Waiting for node registration...")
	return m.waitForNodeReady(hostname)
}

func (m *Manager) waitForNodeReady(hostname string) error {
	kubectlPath := filepath.Join(m.baseDir, "bin", "kubectl")
	maxRetries := 60
	
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command(kubectlPath, "get", "node", hostname, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		output, err := cmd.Output()
		
		if err == nil && strings.TrimSpace(string(output)) == "True" {
			log.Println("  Node is Ready")
			
			labelCmd := exec.Command(kubectlPath, "label", "node", hostname, "node-role.kubernetes.io/master=", "--overwrite")
			if err := labelCmd.Run(); err == nil {
				log.Println("  Node labeled successfully")
			}
			return nil
		}
		
		if i%10 == 0 && i > 0 {
			log.Printf("  Waiting for node to become Ready... (%d/%d)", i, maxRetries)
		}
		time.Sleep(3 * time.Second)
	}
	
	log.Println("  Warning: Node not ready yet, but continuing...")
	return nil
}