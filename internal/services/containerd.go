package services

import (
	"os"
	"os/exec"
)

func (m *Manager) StartContainerd() error {
	cmd := exec.Command("/opt/cni/bin/containerd", "-c", "/etc/containerd/config.toml")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/opt/cni/bin:/usr/sbin")

	return m.startDaemon(cmd, "/var/log/kubernetes/containerd.log")
}