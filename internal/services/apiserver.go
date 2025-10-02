package services

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *Manager) StartAPIServer() error {
	// Проверяем доступность m.hostIP:2379 (etcd)
	etcdEndpoint := "127.0.0.1"
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:2379", m.hostIP), 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		etcdEndpoint = m.hostIP
		log.Printf("  ✓ etcd доступен по %s:2379, используем его", m.hostIP)
	} else {
		log.Printf("  ⚠ etcd по %s:2379 недоступен, переключаемся на 127.0.0.1", m.hostIP)
	}

	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "kube-apiserver"),
		fmt.Sprintf("--etcd-servers=http://%s:2379", etcdEndpoint),
		"--service-cluster-ip-range=10.0.0.0/16", // расширенный диапазон
		"--bind-address=0.0.0.0",
		"--secure-port=6443",
		fmt.Sprintf("--advertise-address=%s", m.hostIP),
		"--authorization-mode=AlwaysAllow",
		"--anonymous-auth=false", // запрещаем анонимные запросы
		"--token-auth-file=/tmp/token.csv",
		"--enable-priority-and-fairness=false",
		"--allow-privileged=true",
		"--profiling=false",
		"--storage-backend=etcd3",
		"--storage-media-type=application/json",
		"--cert-dir=/var/run/kubernetes",    // где будут лежать cert/key
		"--client-ca-file=/tmp/ca.crt",      // CA для клиентов
		"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file=/tmp/sa.pub",
		"--service-account-signing-key-file=/tmp/sa.key",
		"--v=2",
		"--cloud-provider=external",
	)

	if err := m.startDaemon(cmd, "/var/log/kubernetes/apiserver.log"); err != nil {
		return err
	}

	if m.skipAPIWait {
		log.Println("  ⚠️  Skipping API server readiness check (--skip-api-wait enabled)")
		log.Println("  Giving API server 15 seconds to start...")
		time.Sleep(15 * time.Second)
		return nil
	}

	log.Println("  Waiting for API server to become ready (may take up to 10 minutes)...")
	return m.waitForAPIServer()
}

func (m *Manager) waitForAPIServer() error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	maxRetries := 300 // 10 минут (300 попыток × 2с)
	successCount := 0
	requiredSuccesses := 3

	for i := 0; i < maxRetries; i++ {
		urls := []string{
			"https://127.0.0.1:6443/livez",
			"https://127.0.0.1:6443/readyz",
			fmt.Sprintf("https://%s:6443/livez", m.hostIP),
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

		if i%30 == 0 && i > 0 { // раз в минуту пишем прогресс
			log.Printf("  Still waiting... (%d/%d attempts, %d/%d consecutive successes)",
				i, maxRetries, successCount, requiredSuccesses)
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("API server did not become ready in 10 minutes. Check: tail -100 /var/log/kubernetes/apiserver.log")
}
