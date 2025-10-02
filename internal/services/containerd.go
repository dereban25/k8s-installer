package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func (m *Manager) StartContainerd() error {
	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "containerd"),
		"-c", "/etc/containerd/config.toml",
	)

	// расширяем PATH, чтобы находились crictl/ctr
	cmd.Env = append(os.Environ(),
		"PATH="+os.Getenv("PATH")+":"+filepath.Join(m.baseDir, "bin")+":/usr/local/bin:/usr/sbin",
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/containerd.log"); err != nil {
		return err
	}

	log.Println("  Waiting for containerd to become ready...")
	return m.waitForContainerd()
}

func (m *Manager) waitForContainerd() error {
	maxRetries := 60
	if v := os.Getenv("CONTAINERD_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRetries = n
		}
	}

	for i := 0; i < maxRetries; i++ {
		// проверка сокета
		if _, err := os.Stat("/run/containerd/containerd.sock"); err == nil {
			// 1. пробуем crictl
			if _, err := exec.LookPath("crictl"); err == nil {
				if exec.Command("crictl",
					"--runtime-endpoint", "unix:///run/containerd/containerd.sock", "version",
				).Run() == nil {
					log.Println("  ✓ Containerd is ready (crictl)")
					return nil
				}
			}
			// 2. пробуем ctr
			if _, err := exec.LookPath("ctr"); err == nil {
				if exec.Command("ctr",
					"--address", "/run/containerd/containerd.sock", "version",
				).Run() == nil {
					log.Println("  ✓ Containerd is ready (ctr)")
					return nil
				}
			}
			// 3. fallback — сокет есть, процесс жив
			if _, err := exec.Command("pgrep", "-x", "containerd").Output(); err == nil && i > 5 {
				log.Println("  ⚠ No crictl/ctr; socket present and process running — proceeding")
				return nil
			}
		}

		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting for containerd... (%d/%d seconds)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("containerd did not become ready in %d seconds. Check logs: tail -100 /var/log/kubernetes/containerd.log", maxRetries)
}
