package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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
	// ИСПРАВЛЕНО: version 2 с правильной структурой для CRI
	containerdConfig := `version = 2

[grpc]
address = "/run/containerd/containerd.sock"
uid = 0
gid = 0

[debug]
level = "info"

[plugins."io.containerd.grpc.v1.cri"]
sandbox_image = "registry.k8s.io/pause:3.10"

[plugins."io.containerd.grpc.v1.cri".containerd]
snapshotter = "overlayfs"
default_runtime_name = "runc"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
runtime_type = "io.containerd.runc.v2"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
SystemdCgroup = false

[plugins."io.containerd.grpc.v1.cri".cni]
bin_dir = "/opt/cni/bin"
conf_dir = "/etc/cni/net.d"
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

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

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

	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")
	if !exists(kubectlPath) {
		kubectlPath = "kubectl"
	}

	if exists(kubeconfigPath) {
		if err := os.Remove(kubeconfigPath); err != nil {
			fmt.Printf("Warning: couldn't remove old kubeconfig: %v\n", err)
		}
	}

	pkiDir := filepath.Join(i.baseDir, "pki")
	caCandidates := []string{
		filepath.Join(pkiDir, "ca.crt"),
		"/etc/kubernetes/pki/ca.crt",
		"/var/lib/kubernetes/pki/ca.crt",
	}
	adminCrtCandidates := []string{
		filepath.Join(pkiDir, "admin.crt"),
		"/etc/kubernetes/pki/admin.crt",
		"/var/lib/kubernetes/pki/admin.crt",
	}
	adminKeyCandidates := []string{
		filepath.Join(pkiDir, "admin.key"),
		"/etc/kubernetes/pki/admin.key",
		"/var/lib/kubernetes/pki/admin.key",
	}

	caPath, haveCA := waitForFirstExisting(10*time.Second, caCandidates...)
	adminCrt, haveAdminCrt := waitForFirstExisting(5*time.Second, adminCrtCandidates...)
	adminKey, haveAdminKey := waitForFirstExisting(5*time.Second, adminKeyCandidates...)

	if !haveCA {
		return fmt.Errorf("CA certificate not found - cannot configure kubectl securely")
	}

	setClusterArgs := []string{
		"config", "set-cluster", "local-cluster",
		"--server=https://127.0.0.1:6443",
		"--certificate-authority", caPath,
		"--embed-certs=true",
	}
	
	if err := runCommand(kubectlPath, setClusterArgs...); err != nil {
		return fmt.Errorf("failed to configure cluster: %w", err)
	}

	credArgs := []string{"config", "set-credentials", "admin"}
	if haveAdminCrt && haveAdminKey {
		credArgs = append(credArgs,
			"--client-certificate", adminCrt,
			"--client-key", adminKey,
			"--embed-certs=true",
		)
	}

	if err := runCommand(kubectlPath, credArgs...); err != nil {
		return fmt.Errorf("failed to configure credentials: %w", err)
	}

	if err := runCommand(kubectlPath,
		"config", "set-context", "local-context",
		"--cluster=local-cluster",
		"--user=admin",
	); err != nil {
		return fmt.Errorf("failed to set context: %w", err)
	}

	if err := runCommand(kubectlPath, "config", "use-context", "local-context"); err != nil {
		return fmt.Errorf("failed to use context: %w", err)
	}

	kubeconfigData, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	kubeletKubeconfigPath := filepath.Join(i.kubeletDir, "kubeconfig")
	if err := os.WriteFile(kubeletKubeconfigPath, kubeconfigData, 0644); err != nil {
		return fmt.Errorf("failed to write kubelet kubeconfig: %w", err)
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}