package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	k8sVersion         = "v1.30.0"
	containerdVersion  = "2.0.5"
	runcVersion        = "v1.2.6"
	cniPluginsVersion  = "v1.6.2"
	kubebuilderVersion = "1.30.0"
)

type K8sInstaller struct {
	baseDir      string
	etcdDataDir  string
	kubeletDir   string
	manifestsDir string
	cniConfDir   string
	hostIP       string
}

func NewK8sInstaller() (*K8sInstaller, error) {
	hostIP, err := getHostIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get host IP: %w", err)
	}

	return &K8sInstaller{
		baseDir:      "./kubebuilder",
		etcdDataDir:  "./etcd",
		kubeletDir:   "/var/lib/kubelet",
		manifestsDir: "/etc/kubernetes/manifests",
		cniConfDir:   "/etc/cni/net.d",
		hostIP:       hostIP,
	}, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	installer, err := NewK8sInstaller()
	if err != nil {
		log.Fatalf("Failed to create installer: %v", err)
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Creating directories", installer.createDirectories},
		{"Downloading binaries", installer.downloadBinaries},
		{"Generating certificates", installer.generateCertificates},
		{"Creating configurations", installer.createConfigurations},
		{"Starting etcd", installer.startEtcd},
		{"Starting API server", installer.startAPIServer},
		{"Starting containerd", installer.startContainerd},
		{"Configuring kubectl", installer.configureKubectl},
		{"Starting scheduler", installer.startScheduler},
		{"Starting kubelet", installer.startKubelet},
		{"Starting controller manager", installer.startControllerManager},
		{"Creating default resources", installer.createDefaultResources},
		{"Verifying installation", installer.verifyInstallation},
	}

	for _, step := range steps {
		log.Printf("=> %s...", step.name)
		if err := step.fn(); err != nil {
			log.Fatalf("Failed at step '%s': %v", step.name, err)
		}
		log.Printf("✓ %s completed", step.name)
	}

	log.Println("🎉 Kubernetes installation completed successfully!")
	log.Println("You can now use kubectl to interact with your cluster:")
	log.Println("  kubectl get nodes")
	log.Println("  kubectl get pods -A")
}

func (k *K8sInstaller) createDirectories() error {
	dirs := []string{
		filepath.Join(k.baseDir, "bin"),
		k.etcdDataDir,
		k.kubeletDir,
		filepath.Join(k.kubeletDir, "pki"),
		k.manifestsDir,
		"/var/log/kubernetes",
		"/etc/containerd",
		"/run/containerd",
		"/opt/cni/bin",
		k.cniConfDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (k *K8sInstaller) downloadBinaries() error {
	downloads := []struct {
		url      string
		destPath string
		extract  bool
		chmod    bool
	}{
		{
			url:      fmt.Sprintf("https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-%s-linux-amd64.tar.gz", kubebuilderVersion),
			destPath: "/tmp/kubebuilder-tools.tar.gz",
			extract:  true,
		},
		{
			url:      fmt.Sprintf("https://dl.k8s.io/%s/bin/linux/amd64/kubelet", k8sVersion),
			destPath: filepath.Join(k.baseDir, "bin", "kubelet"),
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://dl.k8s.io/%s/bin/linux/amd64/kube-controller-manager", k8sVersion),
			destPath: filepath.Join(k.baseDir, "bin", "kube-controller-manager"),
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://dl.k8s.io/%s/bin/linux/amd64/kube-scheduler", k8sVersion),
			destPath: filepath.Join(k.baseDir, "bin", "kube-scheduler"),
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://github.com/containerd/containerd/releases/download/v%s/containerd-static-%s-linux-amd64.tar.gz", containerdVersion, containerdVersion),
			destPath: "/tmp/containerd.tar.gz",
			extract:  true,
		},
		{
			url:      fmt.Sprintf("https://github.com/opencontainers/runc/releases/download/%s/runc.amd64", runcVersion),
			destPath: "/opt/cni/bin/runc",
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-linux-amd64-%s.tgz", cniPluginsVersion, cniPluginsVersion),
			destPath: "/tmp/cni-plugins.tgz",
			extract:  true,
		},
	}

	for _, dl := range downloads {
		log.Printf("  Downloading %s...", filepath.Base(dl.url))
		if err := downloadFile(dl.url, dl.destPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", dl.url, err)
		}

		if dl.extract {
			if err := k.extractArchive(dl.destPath); err != nil {
				return fmt.Errorf("failed to extract %s: %w", dl.destPath, err)
			}
			os.Remove(dl.destPath)
		}

		if dl.chmod {
			if err := os.Chmod(dl.destPath, 0755); err != nil {
				return fmt.Errorf("failed to chmod %s: %w", dl.destPath, err)
			}
		}
	}

	return nil
}

func (k *K8sInstaller) extractArchive(archivePath string) error {
	var cmd *exec.Cmd

	switch {
	case strings.Contains(archivePath, "kubebuilder-tools"):
		cmd = exec.Command("tar", "-C", k.baseDir, "--strip-components=1", "-zxf", archivePath)
	case strings.Contains(archivePath, "containerd"):
		cmd = exec.Command("tar", "zxf", archivePath, "-C", "/opt/cni/")
	case strings.Contains(archivePath, "cni-plugins"):
		cmd = exec.Command("tar", "zxf", archivePath, "-C", "/opt/cni/bin/")
	default:
		return fmt.Errorf("unknown archive type: %s", archivePath)
	}

	return cmd.Run()
}

func (k *K8sInstaller) generateCertificates() error {
	// Generate CA certificate
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "kubelet-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// Save CA certificate
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})
	if err := os.WriteFile("/tmp/ca.crt", caCertPEM, 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}
	if err := os.WriteFile(filepath.Join(k.kubeletDir, "ca.crt"), caCertPEM, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(k.kubeletDir, "pki", "ca.crt"), caCertPEM, 0644); err != nil {
		return err
	}

	// Save CA key
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)})
	if err := os.WriteFile("/tmp/ca.key", caKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	// Generate service account key pair
	saKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate SA key: %w", err)
	}

	saKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(saKey)})
	if err := os.WriteFile("/tmp/sa.key", saKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write SA key: %w", err)
	}

	saPubPEM, err := x509.MarshalPKIXPublicKey(&saKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal SA public key: %w", err)
	}
	saPubKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: saPubPEM})
	if err := os.WriteFile("/tmp/sa.pub", saPubKeyPEM, 0644); err != nil {
		return fmt.Errorf("failed to write SA public key: %w", err)
	}

	// Create token file
	tokenContent := "1234567890,admin,admin,system:masters\n"
	if err := os.WriteFile("/tmp/token.csv", []byte(tokenContent), 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

func (k *K8sInstaller) createConfigurations() error {
	// CNI configuration
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
	if err := os.WriteFile(filepath.Join(k.cniConfDir, "10-mynet.conf"), []byte(cniConfig), 0644); err != nil {
		return fmt.Errorf("failed to write CNI config: %w", err)
	}

	// Containerd configuration
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
SystemdCgroup = false`
	if err := os.WriteFile("/etc/containerd/config.toml", []byte(containerdConfig), 0644); err != nil {
		return fmt.Errorf("failed to write containerd config: %w", err)
	}

	// Kubelet configuration
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
staticPodPath: "/etc/kubernetes/manifests"`
	if err := os.WriteFile(filepath.Join(k.kubeletDir, "config.yaml"), []byte(kubeletConfig), 0644); err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}

	return nil
}

func (k *K8sInstaller) startEtcd() error {
	cmd := exec.Command(
		filepath.Join(k.baseDir, "bin", "etcd"),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", k.hostIP),
		"--listen-client-urls=http://0.0.0.0:2379",
		fmt.Sprintf("--data-dir=%s", k.etcdDataDir),
		"--listen-peer-urls=http://0.0.0.0:2380",
		fmt.Sprintf("--initial-cluster=default=http://%s:2380", k.hostIP),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", k.hostIP),
		"--initial-cluster-state=new",
		"--initial-cluster-token=test-token",
	)
	
	return k.startDaemon(cmd, "/var/log/kubernetes/etcd.log")
}

func (k *K8sInstaller) startAPIServer() error {
	cmd := exec.Command(
		filepath.Join(k.baseDir, "bin", "kube-apiserver"),
		fmt.Sprintf("--etcd-servers=http://%s:2379", k.hostIP),
		"--service-cluster-ip-range=10.0.0.0/24",
		"--bind-address=0.0.0.0",
		"--secure-port=6443",
		fmt.Sprintf("--advertise-address=%s", k.hostIP),
		"--authorization-mode=AlwaysAllow",
		"--token-auth-file=/tmp/token.csv",
		"--enable-priority-and-fairness=false",
		"--allow-privileged=true",
		"--profiling=false",
		"--storage-backend=etcd3",
		"--storage-media-type=application/json",
		"--v=0",
		"--cloud-provider=external",
		"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file=/tmp/sa.pub",
		"--service-account-signing-key-file=/tmp/sa.key",
	)
	
	if err := k.startDaemon(cmd, "/var/log/kubernetes/apiserver.log"); err != nil {
		return err
	}

	// Wait for API server to be ready
	return k.waitForAPIServer()
}

func (k *K8sInstaller) startContainerd() error {
	cmd := exec.Command("/opt/cni/bin/containerd", "-c", "/etc/containerd/config.toml")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")
	
	return k.startDaemon(cmd, "/var/log/kubernetes/containerd.log")
}

func (k *K8sInstaller) configureKubectl() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	kubeDir := filepath.Join(homeDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	kubectlPath := filepath.Join(k.baseDir, "bin", "kubectl")

	cmds := [][]string{
		{kubectlPath, "config", "set-credentials", "test-user", "--token=1234567890"},
		{kubectlPath, "config", "set-cluster", "test-env", "--server=https://127.0.0.1:6443", "--insecure-skip-tls-verify"},
		{kubectlPath, "config", "set-context", "test-context", "--cluster=test-env", "--user=test-user", "--namespace=default"},
		{kubectlPath, "config", "use-context", "test-context"},
	}

	for _, cmdArgs := range cmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to run kubectl command: %s, output: %s", err, output)
		}
	}

	// Copy kubeconfig for kubelet
	kubeconfigPath := filepath.Join(kubeDir, "config")
	kubeletKubeconfigPath := filepath.Join(k.kubeletDir, "kubeconfig")
	input, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	if err := os.WriteFile(kubeletKubeconfigPath, input, 0644); err != nil {
		return fmt.Errorf("failed to write kubelet kubeconfig: %w", err)
	}

	return nil
}

func (k *K8sInstaller) startScheduler() error {
	cmd := exec.Command(
		filepath.Join(k.baseDir, "bin", "kube-scheduler"),
		fmt.Sprintf("--kubeconfig=%s/.kube/config", os.Getenv("HOME")),
		"--leader-elect=false",
		"--v=2",
		"--bind-address=0.0.0.0",
	)
	
	return k.startDaemon(cmd, "/var/log/kubernetes/scheduler.log")
}

func (k *K8sInstaller) startKubelet() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	cmd := exec.Command(
		filepath.Join(k.baseDir, "bin", "kubelet"),
		fmt.Sprintf("--kubeconfig=%s/kubeconfig", k.kubeletDir),
		fmt.Sprintf("--config=%s/config.yaml", k.kubeletDir),
		fmt.Sprintf("--root-dir=%s", k.kubeletDir),
		fmt.Sprintf("--cert-dir=%s/pki", k.kubeletDir),
		fmt.Sprintf("--hostname-override=%s", hostname),
		"--pod-infra-container-image=registry.k8s.io/pause:3.10",
		fmt.Sprintf("--node-ip=%s", k.hostIP),
		"--cloud-provider=external",
		"--cgroup-driver=cgroupfs",
		"--max-pods=4",
		"--v=1",
	)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")
	
	if err := k.startDaemon(cmd, "/var/log/kubernetes/kubelet.log"); err != nil {
		return err
	}

	// Wait for node to be registered
	time.Sleep(5 * time.Second)

	// Label the node
	kubectlPath := filepath.Join(k.baseDir, "bin", "kubectl")
	cmd = exec.Command(kubectlPath, "label", "node", hostname, "node-role.kubernetes.io/master=", "--overwrite")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: failed to label node: %s, output: %s", err, output)
	}

	return nil
}

func (k *K8sInstaller) startControllerManager() error {
	cmd := exec.Command(
		filepath.Join(k.baseDir, "bin", "kube-controller-manager"),
		fmt.Sprintf("--kubeconfig=%s/kubeconfig", k.kubeletDir),
		"--leader-elect=false",
		"--cloud-provider=external",
		"--service-cluster-ip-range=10.0.0.0/24",
		"--cluster-name=kubernetes",
		fmt.Sprintf("--root-ca-file=%s/ca.crt", k.kubeletDir),
		"--service-account-private-key-file=/tmp/sa.key",
		"--use-service-account-credentials=true",
		"--v=2",
	)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")
	
	return k.startDaemon(cmd, "/var/log/kubernetes/controller-manager.log")
}

func (k *K8sInstaller) createDefaultResources() error {
	time.Sleep(5 * time.Second)

	kubectlPath := filepath.Join(k.baseDir, "bin", "kubectl")
	
	// Create default service account
	cmd := exec.Command(kubectlPath, "create", "sa", "default")
	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			log.Printf("Warning: failed to create default SA: %s, output: %s", err, output)
		}
	}

	// Create kube-root-ca configmap
	cmd = exec.Command(kubectlPath, "create", "configmap", "kube-root-ca.crt", 
		"--from-file=ca.crt=/tmp/ca.crt", "-n", "default")
	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			log.Printf("Warning: failed to create configmap: %s, output: %s", err, output)
		}
	}

	return nil
}

func (k *K8sInstaller) verifyInstallation() error {
	time.Sleep(3 * time.Second)

	kubectlPath := filepath.Join(k.baseDir, "bin", "kubectl")
	
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
			log.Printf("Warning: %s check failed: %s, output: %s", check.name, err, output)
		} else {
			log.Printf("  %s:\n%s", check.name, string(output))
		}
	}

	return nil
}

func (k *K8sInstaller) startDaemon(cmd *exec.Cmd, logFile string) error {
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		logF.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for the process to finish (it's a daemon)
	go func() {
		cmd.Wait()
		logF.Close()
	}()

	return nil
}

func (k *K8sInstaller) waitForAPIServer() error {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(fmt.Sprintf("https://%s:6443/readyz", k.hostIP))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("API server did not become ready in time")
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func getHostIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid IP address found")
}