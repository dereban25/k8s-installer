package installer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dereban25/k8s-installer/internal/services"
)

// Централизованный справочник версий бинарей
const (
	ContainerdVersion  = "2.0.5"
	RuncVersion        = "v1.2.6"
	CNIPluginsVersion  = "v1.6.2"
	KubebuilderVersion = "1.30.0"
	CrictlVersion      = "v1.30.0"
)

// Installer управляет процессом установки Kubernetes
type Installer struct {
	config       *Config
	baseDir      string
	kubeletDir   string
	services     *services.Manager
	etcdDataDir  string
	manifestsDir string
	cniConfDir   string
}

// Config описывает параметры установки
type Config struct {
	K8sVersion      string
	SkipDownload    bool
	SkipVerify      bool
	SkipAPIWait     bool
	ContinueOnError bool
	Verbose         bool
}

// Конструктор: принимает только Config, возвращает Installer + error
func New(cfg *Config) (*Installer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Дефолтные значения
	baseDir := "/var/lib/kubernetes"
	kubeletDir := "/var/lib/kubelet"
	hostIP := "127.0.0.1"

	// Можно переопределить через ENV
	if v := os.Getenv("K8S_BASE_DIR"); v != "" {
		baseDir = v
	}
	if v := os.Getenv("K8S_KUBELET_DIR"); v != "" {
		kubeletDir = v
	}
	if v := os.Getenv("K8S_HOST_IP"); v != "" {
		hostIP = v
	}

	inst := &Installer{
		config:       cfg,
		baseDir:      baseDir,
		kubeletDir:   kubeletDir,
		services:     services.NewManager(baseDir, kubeletDir, hostIP, cfg.SkipAPIWait),
		etcdDataDir:  filepath.Join(baseDir, "etcd"),
		manifestsDir: filepath.Join(baseDir, "manifests"),
		cniConfDir:   "/etc/cni/net.d",
	}
	return inst, nil
}

// Запуск всего пайплайна установки
func (i *Installer) Run() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Creating directories", i.CreateDirectories}, // реализован в directories.go
		{"Downloading binaries", i.DownloadBinaries},
		{"Generating certificates", i.GenerateCertificates},
		{"Creating configurations", i.CreateConfigurations},
		{"Configure kubectl", i.ConfigureKubectl}, // 🔑 добавили сюда
		{"Starting etcd", i.services.StartEtcd},
		{"Starting API server", i.services.StartAPIServer},
		{"Starting containerd", i.services.StartContainerd},
		{"Starting controller-manager", i.services.StartControllerManager},
		{"Starting scheduler", i.services.StartScheduler},
		{"Starting kubelet", i.services.StartKubelet},
	}

	for _, step := range steps {
		log.Printf("=> %s...", step.name)
		if err := step.fn(); err != nil {
			if i.config.ContinueOnError {
				log.Printf("✗ WARNING: %s failed: %v", step.name, err)
				continue
			}
			return fmt.Errorf("failed at step '%s': %w", step.name, err)
		}
		log.Printf("✓ %s completed", step.name)
	}

	return nil
}
