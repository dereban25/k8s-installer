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
	// Сначала убедимся что containerd готов
	if err := m.verifyContainerdCRI(); err != nil {
		return fmt.Errorf("containerd not ready: %w", err)
	}

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
		"--v=2",
	)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	if err := m.startDaemon(cmd, "/var/log/kubernetes/kubelet.log"); err != nil {
		return err
	}

	log.Println("  Waiting for node registration and removing taints...")
	return m.waitForNodeReady(hostname)
}

func (m *Manager) verifyContainerdCRI() error {
	log.Println("  Verifying containerd CRI readiness...")
	maxRetries := 30
	
	for i := 0; i < maxRetries; i++ {
		// Проверяем через crictl
		cmd := exec.Command("crictl", 
			"--runtime-endpoint", "unix:///run/containerd/containerd.sock", 
			"version")
		if err := cmd.Run(); err == nil {
			log.Println("  Containerd CRI is ready")
			return nil
		}
		
		if i%5 == 0 && i > 0 {
			log.Printf("  Waiting for containerd CRI... (%d/%d)", i, maxRetries)
		}
		time.Sleep(2 * time.Second)
	}
	
	return fmt.Errorf("containerd CRI did not respond after %d attempts", maxRetries)
}

func (m *Manager) waitForNodeReady(hostname string) error {
	kubectlPath := filepath.Join(m.baseDir, "bin", "kubectl")
	maxRetries := 60
	
	// Шаг 1: Ждем регистрации ноды
	log.Println("  Waiting for node to register...")
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command(kubectlPath, "get", "node", hostname)
		if err := cmd.Run(); err == nil {
			log.Println("  Node registered in cluster")
			break
		}
		
		if i == maxRetries-1 {
			// Показываем диагностику
			log.Println("  Node registration failed. Diagnostics:")
			diagCmd := exec.Command(kubectlPath, "get", "nodes")
			if output, _ := diagCmd.CombinedOutput(); len(output) > 0 {
				log.Printf("  All nodes: %s", string(output))
			}
			return fmt.Errorf("node %s did not register after %d attempts", hostname, maxRetries)
		}
		
		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting for registration... (%d/%d)", i, maxRetries)
		}
		time.Sleep(3 * time.Second)
	}
	
	// Шаг 2: Убираем taints сразу после регистрации
	log.Println("  Removing taints to allow pod scheduling on control-plane...")
	taints := []string{
		"node-role.kubernetes.io/master:NoSchedule",
		"node-role.kubernetes.io/control-plane:NoSchedule",
		"node.kubernetes.io/not-ready:NoSchedule",
	}
	
	time.Sleep(2 * time.Second) // Небольшая пауза после регистрации
	
	for _, taint := range taints {
		cmd := exec.Command(kubectlPath, "taint", "nodes", hostname, taint+"-")
		output, _ := cmd.CombinedOutput()
		outputStr := string(output)
		
		if strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "not tainted") {
			// Taint уже отсутствует
			continue
		} else if strings.Contains(outputStr, "untainted") {
			log.Printf("  Removed taint: %s", taint)
		}
	}
	
	// Шаг 3: Ждем Ready статус
	log.Println("  Waiting for node Ready status...")
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command(kubectlPath, "get", "node", hostname, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		output, err := cmd.Output()
		
		if err == nil && strings.TrimSpace(string(output)) == "True" {
			log.Println("  Node is Ready")
			
			// Добавляем label
			labelCmd := exec.Command(kubectlPath, "label", "node", hostname, 
				"node-role.kubernetes.io/master=", "--overwrite")
			if err := labelCmd.Run(); err == nil {
				log.Println("  Node labeled as master")
			}
			
			// Финальная проверка что поды могут планироваться
			time.Sleep(3 * time.Second)
			if err := m.verifySchedulable(kubectlPath, hostname); err != nil {
				log.Printf("  Warning: %v", err)
			} else {
				log.Println("  Node is schedulable")
			}
			
			return nil
		}
		
		if i%10 == 0 && i > 0 {
			log.Printf("  Waiting for Ready status... (%d/%d)", i, maxRetries)
		}
		time.Sleep(3 * time.Second)
	}
	
	log.Println("  Warning: Node not Ready yet, but continuing...")
	return nil
}

func (m *Manager) verifySchedulable(kubectlPath, hostname string) error {
	cmd := exec.Command(kubectlPath, "get", "node", hostname, 
		"-o", "jsonpath={.spec.taints}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check taints: %w", err)
	}
	
	taintsStr := strings.TrimSpace(string(output))
	if taintsStr != "" && taintsStr != "null" && taintsStr != "[]" {
		// Проверяем есть ли NoSchedule taints
		if strings.Contains(taintsStr, "NoSchedule") {
			return fmt.Errorf("node still has NoSchedule taints: %s", taintsStr)
		}
	}
	
	return nil
}