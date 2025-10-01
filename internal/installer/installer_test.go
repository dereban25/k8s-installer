package installer

import (
	"testing"
)

func TestNew(t *testing.T) {
	cfg := &Config{
		K8sVersion:   "v1.30.0",
		SkipDownload: false,
		SkipVerify:   false,
		Verbose:      false,
	}

	inst, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if inst == nil {
		t.Fatal("Installer is nil")
	}

	if inst.baseDir != "./kubebuilder" {
		t.Errorf("Expected baseDir to be './kubebuilder', got '%s'", inst.baseDir)
	}

	if inst.config.K8sVersion != "v1.30.0" {
		t.Errorf("Expected K8sVersion to be 'v1.30.0', got '%s'", inst.config.K8sVersion)
	}
}

func TestGetters(t *testing.T) {
	cfg := &Config{
		K8sVersion: "v1.30.0",
	}

	inst, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if inst.GetBaseDir() != "./kubebuilder" {
		t.Errorf("GetBaseDir() failed")
	}

	if inst.GetKubeletDir() != "/var/lib/kubelet" {
		t.Errorf("GetKubeletDir() failed")
	}

	if inst.GetHostIP() == "" {
		t.Errorf("GetHostIP() returned empty string")
	}
}