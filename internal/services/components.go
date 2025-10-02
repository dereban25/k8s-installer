package services

import (
	"fmt"
	"log"
	"os/exec"
)

// StartEtcd запускает etcd
func (m *Manager) StartEtcd() error {
	log.Println("  => Запускаем etcd...")
	cmd := exec.Command("/usr/local/bin/etcd")
	return m.startDaemon(cmd, "/var/log/kubernetes/etcd.log")
}

// StartAPIServer запускает kube-apiserver
func (m *Manager) StartAPIServer() error {
	log.Println("  => Запускаем kube-apiserver...")
	cmd := exec.Command("/usr/local/bin/kube-apiserver", "--config=/etc/kubernetes/manifests/apiserver.yaml")
	return m.startDaemon(cmd, "/var/log/kubernetes/apiserver.log")
}

// StartScheduler запускает kube-scheduler
func (m *Manager) StartScheduler() error {
	log.Println("  => Запускаем kube-scheduler...")
	cmd := exec.Command("/usr/local/bin/kube-scheduler", "--config=/etc/kubernetes/manifests/scheduler.yaml")
	return m.startDaemon(cmd, "/var/log/kubernetes/scheduler.log")
}

// StartKubelet запускает kubelet
func (m *Manager) StartKubelet() error {
	log.Println("  => Запускаем kubelet...")
	cmd := exec.Command("/usr/local/bin/kubelet", "--config=/var/lib/kubelet/config.yaml")
	return m.startDaemon(cmd, "/var/log/kubernetes/kubelet.log")
}

// StartControllerManager запускает kube-controller-manager
func (m *Manager) StartControllerManager() error {
	log.Println("  => Запускаем kube-controller-manager...")
	cmd := exec.Command("/usr/local/bin/kube-controller-manager", "--config=/etc/kubernetes/manifests/controller-manager.yaml")
	return m.startDaemon(cmd, "/var/log/kubernetes/controller-manager.log")
}

// Utility: проверка бинарей
func (m *Manager) checkBinaryExists(path string) error {
	if _, err := exec.LookPath(path); err != nil {
		return fmt.Errorf("бинарь %s не найден в PATH", path)
	}
	return nil
}
