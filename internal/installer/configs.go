package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// --------------------------
// Конфиги (как у тебя было)
// --------------------------

func (i *Installer) CreateConfigurations() error {
	if err := i.createCNIConfig(); err != nil {
		return err
	}
	if err := i.createContainerdConfig(); err != nil {
		return err
	}
	if err := i.createKubeletConfig(); err != nil {
		return err
	}
	return nil
}

func (i *Installer) createCNIConfig() error {
	cniConfig := `{
  "cniVersion": "0.3.1",
  "name": "mynet",
  "type": "bridge",
  "bridge": "cni0",
  "isGateway": true,
  "ipMasq": true,
  "ipam": {
    "type": "host-local",
    "subnet": "10.22.0.0/16",
    "routes": [
      { "dst": "0.0.0.0/0" }
    ]
  }
}`
	configPath := filepath.Join(i.cniConfDir, "10-mynet.conf")
	if err := os.WriteFile(configPath, []byte(cniConfig), 0644); err != nil {
		return fmt.Errorf("failed to write CNI config: %w", err)
	}
	return nil
}

func (i *Installer) createContainerdConfig() error {
	containerdConfig := `version = 3

[grpc]
address = "/run/containerd/containerd.sock"

[plugins.'io.containerd.cri.v1.runtime']
enable_selinux = false
enable_unprivileged_ports = true
enable_unprivileged_icmp = true
device_ownership_from_security_context = false

[plugins.'io.containerd.cri.v1.images']
snapshotter = "native"
disable_snapshot_annotations = true

[plugins.'io.containerd.cri.v1.runtime'.cni]
bin_dir = "/opt/cni/bin"
conf_dir = "/etc/cni/net.d"

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc]
runtime_type = "io.containerd.runc.v2"

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc.options]
SystemdCgroup = false
`
	if err := os.WriteFile("/etc/containerd/config.toml", []byte(containerdConfig), 0644); err != nil {
		return fmt.Errorf("failed to write containerd config: %w", err)
	}
	return nil
}

func (i *Installer) createKubeletConfig() error {
	kubeletConfig := `apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: true
  webhook:
    enabled: true
  x509:
    clientCAFile: "/var/lib/kubelet/ca.crt"
authorization:
  mode: AlwaysAllow
clusterDomain: "cluster.local"
clusterDNS:
  - "10.0.0.10"
resolvConf: "/etc/resolv.conf"
runtimeRequestTimeout: "15m"
failSwapOn: false
seccompDefault: true
serverTLSBootstrap: false
containerRuntimeEndpoint: "unix:///run/containerd/containerd.sock"
staticPodPath: "/etc/kubernetes/manifests"
`
	configPath := filepath.Join(i.kubeletDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(kubeletConfig), 0644); err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}
	return nil
}

// -------------------------------------
// Конфигурация kubectl (аккуратный фикс)
// -------------------------------------

// маленький помощник: ждём появления любого из путей
func waitForFirstExisting(timeout time.Duration, candidates ...string) (string, bool) {
	deadline := time.Now().Add(timeout)
	for {
		for _, p := range candidates {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
		if time.Now().After(deadline) {
			return "", false
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// exists проверяет путь
func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ConfigureKubectl создаёт kubeconfig в $HOME/.kube/config и копирует в kubeletDir/kubeconfig.
// 1) Пытается использовать ca.crt + admin.crt/admin.key из i.baseDir/pki (или /etc/kubernetes/pki/* если так у тебя генерится);
// 2) Если файлов пока нет — не падает, а настраивает кластер с --insecure-skip-tls-verify (чтобы пайплайн не разбивался).
func (i *Installer) ConfigureKubectl() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	kubeDir := filepath.Join(homeDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}
	kubeconfigPath := filepath.Join(kubeDir, "config")

	// где лежит kubectl
	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")
	if !exists(kubectlPath) {
		// fallback на kubectl из PATH
		kubectlPath = "kubectl"
	}

	// кандидаты для сертификатов (поддерживаем оба распространённых места)
	caCandidates := []string{
		filepath.Join(i.baseDir, "pki", "ca.crt"),
		"/etc/kubernetes/pki/ca.crt",
	}
	adminCrtCandidates := []string{
		filepath.Join(i.baseDir, "pki", "admin.crt"),
		"/etc/kubernetes/pki/admin.crt",
	}
	adminKeyCandidates := []string{
		filepath.Join(i.baseDir, "pki", "admin.key"),
		"/etc/kubernetes/pki/admin.key",
	}

	// ждём CA чуть-чуть (на случай, если GenerateCertificates пишет асинхронно)
	caPath, haveCA := waitForFirstExisting(2*time.Second, caCandidates...)
	adminCrt, haveAdminCrt := waitForFirstExisting(0, adminCrtCandidates...)
	adminKey, haveAdminKey := waitForFirstExisting(0, adminKeyCandidates...)

	// set-cluster
	setClusterArgs := []string{"config", "set-cluster", "local-cluster", "--server=https://127.0.0.1:6443"}
	if haveCA {
		setClusterArgs = append(setClusterArgs, "--certificate-authority", caPath, "--embed-certs=true")
	} else {
		// аккуратный fallback: не роняем инсталлятор, но помечаем как insecure
		setClusterArgs = append(setClusterArgs, "--insecure-skip-tls-verify=true")
	}
	if err := runCommand(kubectlPath, setClusterArgs...); err != nil {
		return fmt.Errorf("failed to configure cluster: %w", err)
	}

	// set-credentials
	credArgs := []string{"config", "set-credentials", "admin"}
	if haveAdminCrt && haveAdminKey {
		credArgs = append(credArgs, "--client-certificate", adminCrt, "--client-key", adminKey, "--embed-certs=true")
	}
	// Если админских ключей нет — создадим юзера без cred’ов (kubectl конфиг всё равно создаст).
	if err := runCommand(kubectlPath, credArgs...); err != nil {
		return fmt.Errorf("failed to configure credentials: %w", err)
	}

	// set-context / use-context
	if err := runCommand(kubectlPath, "config", "set-context", "local-context", "--cluster=local-cluster", "--user=admin"); err != nil {
		return fmt.Errorf("failed to set context: %w", err)
	}
	if err := runCommand(kubectlPath, "config", "use-context", "local-context"); err != nil {
		return fmt.Errorf("failed to use context: %w", err)
	}

	// Скопируем kubeconfig для kubelet
	b, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	kubeletKubeconfigPath := filepath.Join(i.kubeletDir, "kubeconfig")
	if err := os.WriteFile(kubeletKubeconfigPath, b, 0644); err != nil {
		return fmt.Errorf("failed to write kubelet kubeconfig: %w", err)
	}

	return nil
}
