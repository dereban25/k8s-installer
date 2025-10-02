package installer

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TestAPIServerConnection проверяет TCP-подключение к API server
func (i *Installer) TestAPIServerConnection() error {
	log.Println("🔍 Testing API server connectivity...")
	
	addresses := []string{
		"127.0.0.1:6443",
		"localhost:6443",
	}
	
	// Увеличиваем количество попыток и время ожидания
	maxAttempts := 30
	retryDelay := 2 * time.Second
	
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		for _, addr := range addresses {
			if attempt == 1 || attempt%5 == 0 {
				log.Printf("  Attempt %d/%d: trying %s...", attempt, maxAttempts, addr)
			}
			
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err == nil {
				conn.Close()
				log.Printf("  ✓ Connected to %s after %d attempts", addr, attempt)
				return nil
			}
		}
		
		if attempt < maxAttempts {
			time.Sleep(retryDelay)
		}
	}
	
	log.Println("  ✗ Failed to connect after all attempts")
	log.Println("")
	log.Println("=== Diagnostics ===")
	
	// Проверяем процесс
	if exec.Command("pgrep", "kube-apiserver").Run() == nil {
		log.Println("  ⚠️  API server process is running but not listening on port 6443")
		log.Println("  This might be a configuration or certificate issue")
		
		// Показываем логи если возможно
		log.Println("")
		log.Println("=== API Server Logs (last 30 lines) ===")
		cmd := exec.Command("journalctl", "-u", "kube-apiserver", "--no-pager", "-n", "30")
		if output, err := cmd.CombinedOutput(); err == nil {
			log.Printf("%s", string(output))
		} else {
			// Пробуем через ps
			log.Println("journalctl not available, showing process info:")
			psCmd := exec.Command("ps", "aux")
			if psOutput, err := psCmd.CombinedOutput(); err == nil {
				lines := strings.Split(string(psOutput), "\n")
				for _, line := range lines {
					if strings.Contains(line, "kube-apiserver") {
						log.Println(line)
					}
				}
			}
		}
	} else {
		log.Println("  ✗ API server process is not running at all")
	}
	
	return fmt.Errorf("cannot establish TCP connection to API server after %d attempts", maxAttempts)
}

// VerifyKubeconfigSetup проверяет корректность kubeconfig
func (i *Installer) VerifyKubeconfigSetup() error {
	log.Println("🔍 Verifying kubeconfig...")
	
	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")
	
	// Проверяем текущий контекст
	cmd := exec.Command(kubectlPath, "config", "current-context")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w\nOutput: %s", err, string(output))
	}
	log.Printf("  ✓ Current context: %s", strings.TrimSpace(string(output)))
	
	// Проверяем server URL
	cmd = exec.Command(kubectlPath, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get cluster server: %w", err)
	}
	
	server := strings.TrimSpace(string(output))
	log.Printf("  ✓ Cluster server: %s", server)
	
	if !strings.Contains(server, "6443") {
		return fmt.Errorf("❌ kubeconfig uses wrong port! Expected 6443, got: %s", server)
	}
	
	if !strings.HasPrefix(server, "https://") {
		return fmt.Errorf("❌ kubeconfig uses insecure connection! Expected https://, got: %s", server)
	}
	
	log.Println("  ✓ kubeconfig properly configured (HTTPS on port 6443)")
	return nil
}

func (i *Installer) CreateDefaultResources() error {
	log.Println("📦 Creating default resources...")
	time.Sleep(5 * time.Second)

	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")

	// Создаем default service account
	if err := runCommandWithCheck(kubectlPath, "create", "sa", "default"); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Printf("⚠️  Warning: failed to create default SA: %v", err)
		} else {
			log.Println("  ℹ️  Default service account already exists")
		}
	} else {
		log.Println("  ✓ Created default service account")
	}

	// Создаем kube-root-ca configmap
	caPath := filepath.Join(i.baseDir, "pki", "ca.crt")
	if err := runCommandWithCheck(kubectlPath, "create", "configmap", "kube-root-ca.crt",
		fmt.Sprintf("--from-file=ca.crt=%s", caPath), "-n", "default"); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Printf("⚠️  Warning: failed to create configmap: %v", err)
		} else {
			log.Println("  ℹ️  kube-root-ca.crt already exists")
		}
	} else {
		log.Println("  ✓ Created kube-root-ca.crt configmap")
	}

	return nil
}

func (i *Installer) VerifyInstallation() error {
	log.Println("🔍 Verifying installation...")
	
	// Проверяем TCP подключение
	if err := i.TestAPIServerConnection(); err != nil {
		return fmt.Errorf("API server connectivity test failed: %w", err)
	}
	
	// Проверяем kubeconfig
	if err := i.VerifyKubeconfigSetup(); err != nil {
		return fmt.Errorf("kubeconfig verification failed: %w", err)
	}
	
	time.Sleep(3 * time.Second)

	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")

	checks := []struct {
		args        []string
		name        string
		critical    bool
		retryCount  int
		retryDelay  time.Duration
	}{
		{
			args:       []string{"version", "--client"},
			name:       "kubectl version",
			critical:   true,
			retryCount: 3,
			retryDelay: 2 * time.Second,
		},
		{
			args:       []string{"get", "--raw=/healthz"},
			name:       "API health",
			critical:   true,
			retryCount: 5,
			retryDelay: 3 * time.Second,
		},
		{
			args:       []string{"get", "--raw=/readyz?verbose"},
			name:       "API readiness",
			critical:   false,
			retryCount: 3,
			retryDelay: 2 * time.Second,
		},
		{
			args:       []string{"get", "nodes"},
			name:       "nodes",
			critical:   false,
			retryCount: 3,
			retryDelay: 2 * time.Second,
		},
		{
			args:       []string{"get", "componentstatuses"},
			name:       "component statuses",
			critical:   false,
			retryCount: 2,
			retryDelay: 2 * time.Second,
		},
		{
			args:       []string{"get", "pods", "-A"},
			name:       "all pods",
			critical:   false,
			retryCount: 2,
			retryDelay: 2 * time.Second,
		},
	}

	allPassed := true
	for _, check := range checks {
		log.Printf("  Checking %s...", check.name)

		var lastErr error
		var lastOutput []byte
		
		for attempt := 0; attempt <= check.retryCount; attempt++ {
			if attempt > 0 {
				log.Printf("    Retry %d/%d...", attempt, check.retryCount)
				time.Sleep(check.retryDelay)
			}
			
			cmd := exec.Command(kubectlPath, check.args...)
			output, err := cmd.CombinedOutput()
			
			if err == nil {
				log.Printf("  ✓ %s check passed", check.name)
				if i.config.Verbose {
					log.Printf("Output:\n%s", string(output))
				}
				break
			}
			
			lastErr = err
			lastOutput = output
			
			if attempt == check.retryCount {
				if check.critical {
					return fmt.Errorf("critical check '%s' failed: %w\nOutput: %s", 
						check.name, lastErr, string(lastOutput))
				}
				log.Printf("  ✗ %s check failed: %v", check.name, lastErr)
				if i.config.Verbose {
					log.Printf("Output: %s", string(lastOutput))
				}
				allPassed = false
			}
		}
	}

	if !allPassed {
		log.Println("⚠️  Some non-critical checks failed")
	} else {
		log.Println("✅ All checks passed!")
	}

	return nil
}

func runCommandWithCheck(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}