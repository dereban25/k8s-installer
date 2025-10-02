package installer

import (
	"fmt"
	"os"
	"path/filepath"
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

// ✅ Новый ConfigureKubectl с сертификатами
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
	caCert := filepath.Join(i.baseDir, "pki", "ca.crt")
	adminCert := filepath.Join(i.baseDir, "pki", "admin.crt")
	adminKey := filepath.Join(i.baseDir, "pki", "admin.key")

	kubectlPath := filepath.Join(i.baseDir, "bin", "kubectl")

	commands := [][]string{
		{kubectlPath, "config", "set-cluster", "local-cluster",
			"--server=https://127.0.0.1:6443",
			"--certificate-authority", caCert,
			"--embed-certs=true"},
		{kubectlPath, "config", "set-credentials", "admin",
			"--client-certificate", adminCert,
			"--client-key", adminKey,
			"--embed-certs=true"},
		{kubectlPath, "config", "set-context", "local-context",
			"--cluster=local-cluster",
			"--user=admin"},
		{kubectlPath, "config", "use-context", "local-context"},
	}

	for _, cmd := range commands {
		if err := runCommand(cmd[0], cmd[1:]...); err != nil {
			return fmt.Errorf("failed to configure kubectl: %w", err)
		}
	}

	// Копия для kubelet
	input, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	kubeletKubeconfigPath := filepath.Join(i.kubeletDir, "kubeconfig")
	if err := os.WriteFile(kubeletKubeconfigPath, input, 0644); err != nil {
		return fmt.Errorf("failed to write kubelet kubeconfig: %w", err)
	}

	return nil
}
