package services

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	baseDir := "./kubebuilder"
	kubeletDir := "/var/lib/kubelet"
	hostIP := "192.168.1.1"
	skipAPIWait := false

	mgr := NewManager(baseDir, kubeletDir, hostIP, skipAPIWait)

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

	if mgr.skipAPIWait != skipAPIWait {
		t.Errorf("Expected skipAPIWait to be %v, got %v", skipAPIWait, mgr.skipAPIWait)
	}
}

func TestNewManagerWithSkipAPIWait(t *testing.T) {
	baseDir := "./kubebuilder"
	kubeletDir := "/var/lib/kubelet"
	hostIP := "192.168.1.1"
	skipAPIWait := true

	mgr := NewManager(baseDir, kubeletDir, hostIP, skipAPIWait)

	if mgr == nil {
		t.Fatal("Manager is nil")
	}

	if !mgr.skipAPIWait {
		t.Errorf("Expected skipAPIWait to be true, got false")
	}
}