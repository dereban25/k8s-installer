package installer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (i *Installer) CreateDefaultResources() error {
	time.Sleep(5 * time.Second)

	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")

	// Create default service account
	if err := runCommandWithCheck(kubectlPath, "create", "sa", "default"); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Printf("Warning: failed to create default SA: %v", err)
		}
	}

	// Create kube-root-ca configmap
	if err := runCommandWithCheck(kubectlPath, "create", "configmap", "kube-root-ca.crt",
		"--from-file=ca.crt=/tmp/ca.crt", "-n", "default"); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Printf("Warning: failed to create configmap: %v", err)
		}
	}

	return nil
}

func (i *Installer) VerifyInstallation() error {
	time.Sleep(3 * time.Second)

	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")

	checks := []struct {
		args []string
		name string
	}{
		{[]string{"get", "nodes"}, "nodes"},
		{[]string{"get", "componentstatuses"}, "component statuses"},
		{[]string{"get", "--raw=/readyz?verbose"}, "API server health"},
	}

	for _, check := range checks {
		log.Printf("  Checking %s...", check.name)
		
		cmd := exec.Command(kubectlPath, check.args...)
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			log.Printf("Warning: %s check failed: %v", check.name, err)
			if i.config.Verbose {
				log.Printf("Output: %s", string(output))
			}
		} else {
			log.Printf("✓ %s check passed", check.name)
			if i.config.Verbose {
				log.Printf("%s:\n%s", check.name, string(output))
			}
		}
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, output)
	}
	return nil
}

func runCommandWithCheck(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}