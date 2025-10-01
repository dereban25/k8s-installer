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
		"--max-pods=4",
		"--v=1",
	)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	if err := m.startDaemon(cmd, "/var/log/kubernetes/kubelet.log"); err != nil {
		return err
	}

	// Wait longer for node registration
	log.Println("  Waiting for node registration...")
	time.Sleep(15 * time.Second)

	// Label the node with retries
	kubectlPath := filepath.Join(m.baseDir, "bin", "kubectl")
	for retry := 0; retry < 5; retry++ {
		labelCmd := exec.Command(kubectlPath, "label", "node", hostname, "node-role.kubernetes.io/master=", "--overwrite")
		output, err := labelCmd.CombinedOutput()
		if err == nil || strings.Contains(string(output), "already has") {
			log.Println("  ✓ Node labeled successfully")
			return nil
		}
		if retry < 4 {
			log.Printf("  Retrying node labeling... (%d/5)", retry+2)
			time.Sleep(5 * time.Second)
		}
	}

	log.Println("  Warning: Failed to label node, but continuing...")
	return nil
}