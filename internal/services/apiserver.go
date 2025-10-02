package services

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *Manager) StartAPIServer() error {
	// По умолчанию etcd = localhost
	etcdEndpoint := "127.0.0.1"

	// Проверим health у etcd по hostIP
	url := fmt.Sprintf("http://%s:2379/health", m.hostIP)
	client := &http.Client{Timeout: 1 * time.Second}

	if resp, err := client.Get(url); err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 200 && string(body) != "" {
			etcdEndpoint = m.hostIP
			log.Printf("  ✓ etcd доступен по %s:2379 (health-check ok), используем его", m.hostIP)
		} else {
			log.Printf("  ⚠ etcd health-check по %s:2379 вернул %d (%s), fallback на 127.0.0.1",
				m.hostIP, resp.StatusCode, string(body))
		}
	} else {
		log.Printf("  ⚠ etcd health-check по %s:2379 не прошёл (%v), fallback на 127.0.0.1",
			m.hostIP, err)
	}

	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "kube-apiserver"),
		fmt.Sprintf("--etcd-servers=http://%s:2379", etcdEndpoint),
		"--service-cluster-ip-range=10.0.0.0/16",
		"--bind-address=0.0.0.0",             // ✅ слушаем на всех интерфейсах
		"--secure-port=6443",
		"--insecure-port=8080",               // для диагностики
		"--insecure-bind-address=0.0.0.0",
		fmt.Sprintf("--advertise-address=%s", m.hostIP),
		"--authorization-mode=AlwaysAllow",
		"--anonymous-auth=false",
		"--token-auth-file=/tmp/token.csv",
		"--enable-priority-and-fairness=false",
		"--allow-privileged=true",
		"--profiling=false",
		"--storage-backend=etcd3",
		"--storage-media-type=application/json",
		"--cert-dir=/var/run/kubernetes",
		"--client-ca-file=/tmp/ca.crt",
		"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file=/tmp/sa.pub",
		"--service-account-signing-key-file=/tmp/sa.key",
		"--cloud-provider=external",
		"--v=5", // подробные логи
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/apiserver.log"); err != nil {
		return err
	}

	if m.skipAPIWait {
		log.Println("  ⚠️ Skipping API server readiness check (--skip-api-wait enabled)")
		log.Println("  Giving API server 15 seconds to start...")
		time.Sleep(15 * time.Second)
		return nil
	}

	log.Println("  Waiting for API server to become ready (up to 10 minutes)...")
	return m.waitForAPIServer()
}

func (m *Manager) waitForAPIServer() error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	maxRetries := 300 // 10 минут
	successCount := 0
	requiredSuccesses := 3

	for i := 0; i < maxRetries; i++ {
		urls := []string{
			"https://127.0.0.1:6443/livez", // ✅ всегда проверяем localhost
			"https://127.0.0.1:6443/readyz",
			fmt.Sprintf("https://%s:6443/livez", m.hostIP),
			"http://127.0.0.1:8080/healthz", // fallback для отладки
		}

		success := false
		for _, url := range urls {
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					success = true
					break
				}
			}
		}

		if success {
			successCount++
			if successCount >= requiredSuccesses {
				log.Printf("  ✓ API server is ready")
				time.Sleep(3 * time.Second)
				return nil
			}
		} else {
			successCount = 0
		}

		if i%30 == 0 && i > 0 { // раз в минуту показываем прогресс
			log.Printf("  Still waiting... (%d/%d attempts, %d/%d consecutive successes)",
				i, maxRetries, successCount, requiredSuccesses)
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("API server did not become ready in 10 minutes. Check: tail -100 /var/log/kubernetes/apiserver.log")
}
