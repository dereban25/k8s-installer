package services

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	baseDir := "./kubebuilder"
	kubeletDir := "/var/lib/kubelet"
	hostIP := "192.168.1.1"

	mgr := NewManager(baseDir, kubeletDir, hostIP)

	if mgr == nil {
		t.Fatal("Manager is nil")
	}

	if mgr.baseDir != baseDir {
		t.Errorf("Expected baseDir to be '%s', got '%s'", baseDir, mgr.baseDir)
	}

	if mgr.kubeletDir != kubeletDir {
		t.Errorf("Expected kubeletDir to be '%s', got '%s'", kubeletDir, mgr.kubeletDir)
	}

	if mgr.hostIP != hostIP {
		t.Errorf("Expected hostIP to be '%s', got '%s'", hostIP, mgr.hostIP)
	}
}