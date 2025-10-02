package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (m *Manager) StartControllerManager() error {
	pkiDir := filepath.Join(m.baseDir, "pki")
	
	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "kube-controller-manager"),
		fmt.Sprintf("--kubeconfig=%s/kubeconfig", m.kubeletDir),
		"--leader-elect=false",
		"--cloud-provider=external",
		"--service-cluster-ip-range=10.0.0.0/24",
		"--cluster-name=kubernetes",
		fmt.Sprintf("--root-ca-file=%s/ca.crt", pkiDir),
		fmt.Sprintf("--service-account-private-key-file=%s/sa.key", pkiDir),
		"--use-service-account-credentials=true",
		"--v=2",
	)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	return m.startDaemon(cmd, "/var/log/kubernetes/controller-manager.log")
}