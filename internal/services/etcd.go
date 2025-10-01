package services

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func (m *Manager) StartEtcd() error {
	cmd := exec.Command(
		filepath.Join(m.baseDir, "bin", "etcd"),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", m.hostIP),
		"--listen-client-urls=http://0.0.0.0:2379",
		"--data-dir=./etcd",
		"--listen-peer-urls=http://0.0.0.0:2380",
		fmt.Sprintf("--initial-cluster=default=http://%s:2380", m.hostIP),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", m.hostIP),
		"--initial-cluster-state=new",
		"--initial-cluster-token=test-token",
	)

	return m.startDaemon(cmd, "/var/log/kubernetes/etcd.log")
}