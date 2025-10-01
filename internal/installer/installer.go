package installer

import (
	"fmt"
	"log"

	"github.com/yourname/k8s-installer/internal/services"
	"github.com/yourname/k8s-installer/internal/utils"
)

const (
	ContainerdVersion  = "2.0.5"
	RuncVersion        = "v1.2.6"
	CNIPluginsVersion  = "v1.6.2"
	KubebuilderVersion = "1.30.0"
)

type Config struct {
	K8sVersion      string
	SkipDownload    bool
	SkipVerify      bool
	SkipAPIWait     bool
	ContinueOnError bool
	Verbose         bool
}

type Installer struct {
	config       *Config
	baseDir      string
	etcdDataDir  string
	kubeletDir   string
	manifestsDir string
	cniConfDir   string
	hostIP       string
	services     *services.Manager
}

func New(cfg *Config) (*Installer, error) {
	hostIP, err := utils.GetHostIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get host IP: %w", err)
	}

	inst := &Installer{
		config:       cfg,
		baseDir:      "./kubebuilder",
		etcdDataDir:  "./etcd",
		kubeletDir:   "/var/lib/kubelet",
		manifestsDir: "/etc/kubernetes/manifests",
		cniConfDir:   "/etc/cni/net.d",
		hostIP:       hostIP,
	}

	inst.services = services.NewManager(inst.baseDir, inst.kubeletDir, inst.hostIP, inst.config.SkipAPIWait)

	return inst, nil
}

func (i *Installer) Run() error {
	steps := []struct {
		name     string
		skip     bool
		critical bool
		fn       func() error
	}{
		{"Creating directories", false, true, i.CreateDirectories},
		{"Downloading binaries", i.config.SkipDownload, true, i.DownloadBinaries},
		{"Generating certificates", false, true, i.GenerateCertificates},
		{"Creating configurations", false, true, i.CreateConfigurations},
		{"Starting etcd", false, true, i.services.StartEtcd},
		{"Starting API server", false, true, i.services.StartAPIServer},
		{"Creating system namespaces", false, false, i.services.CreateSystemNamespaces},
		{"Starting containerd", false, true, i.services.StartContainerd},
		{"Configuring kubectl", false, true, i.ConfigureKubectl},
		{"Starting scheduler", false, false, i.services.StartScheduler},
		{"Starting kubelet", false, false, i.services.StartKubelet},
		{"Starting controller manager", false, false, i.services.StartControllerManager},
		{"Creating default resources", false, false, i.CreateDefaultResources},
		{"Verifying installation", i.config.SkipVerify, false, i.VerifyInstallation},
	}

	for _, step := range steps {
		if step.skip {
			log.Printf("⊘ Skipping: %s", step.name)
			continue
		}

		log.Printf("=> %s...", step.name)
		if err := step.fn(); err != nil {
			if step.critical {
				log.Printf("✗ CRITICAL: %s failed", step.name)
				return fmt.Errorf("failed at step '%s': %w\n\nCheck logs at /var/log/kubernetes/", step.name, err)
			}
			log.Printf("⚠ Warning: %s failed (non-critical): %v", step.name, err)
			continue
		}
		log.Printf("✓ %s completed", step.name)
	}

	return nil
}

func (i *Installer) GetBaseDir() string {
	return i.baseDir
}

func (i *Installer) GetHostIP() string {
	return i.hostIP
}

func (i *Installer) GetKubeletDir() string {
	return i.kubeletDir
}