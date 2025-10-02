package installer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dereban25/k8s-installer/internal/utils"
)

type download struct {
	url      string
	destPath string
	extract  bool
	chmod    bool
}

func (i *Installer) DownloadBinaries() error {
	downloads := []download{
		{
			url:      fmt.Sprintf("https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-%s-linux-amd64.tar.gz", KubebuilderVersion),
			destPath: "/tmp/kubebuilder-tools.tar.gz",
			extract:  true,
		},
		{
			url:      fmt.Sprintf("https://dl.k8s.io/%s/bin/linux/amd64/kubelet", i.config.K8sVersion),
			destPath: filepath.Join(i.baseDir, "bin", "kubelet"),
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://dl.k8s.io/%s/bin/linux/amd64/kube-controller-manager", i.config.K8sVersion),
			destPath: filepath.Join(i.baseDir, "bin", "kube-controller-manager"),
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://dl.k8s.io/%s/bin/linux/amd64/kube-scheduler", i.config.K8sVersion),
			destPath: filepath.Join(i.baseDir, "bin", "kube-scheduler"),
			chmod:    true,
		},
		// ✅ containerd
		{
			url:      fmt.Sprintf("https://github.com/containerd/containerd/releases/download/v%s/containerd-%s-linux-amd64.tar.gz", ContainerdVersion, ContainerdVersion),
			destPath: "/tmp/containerd.tar.gz",
			extract:  true,
		},
		{
			url:      fmt.Sprintf("https://github.com/opencontainers/runc/releases/download/%s/runc.amd64", RuncVersion),
			destPath: filepath.Join(i.baseDir, "bin", "runc"),
			chmod:    true,
		},
		{
			url:      fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-linux-amd64-%s.tgz", CNIPluginsVersion, CNIPluginsVersion),
			destPath: "/tmp/cni-plugins.tgz",
			extract:  true,
		},
		{
			url:      fmt.Sprintf("https://github.com/kubernetes-sigs/cri-tools/releases/download/%s/crictl-%s-linux-amd64.tar.gz", CrictlVersion, CrictlVersion),
			destPath: "/tmp/crictl.tar.gz",
			extract:  true,
		},
	}

	for _, dl := range downloads {
		log.Printf("  Downloading %s...", filepath.Base(dl.url))

		if err := utils.DownloadFile(dl.url, dl.destPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", dl.url, err)
		}

		if dl.extract {
			if err := i.extractArchive(dl.destPath); err != nil {
				return fmt.Errorf("failed to extract %s: %w", dl.destPath, err)
			}
			_ = os.Remove(dl.destPath)
		}

		if dl.chmod {
			if err := os.Chmod(dl.destPath, 0755); err != nil {
				return fmt.Errorf("failed to chmod %s: %w", dl.destPath, err)
			}
		}
	}

	return nil
}

func (i *Installer) extractArchive(archivePath string) error {
	var cmd *exec.Cmd

	switch {
	case strings.Contains(archivePath, "kubebuilder-tools"):
		cmd = exec.Command("tar", "-C", i.baseDir, "--strip-components=1", "-zxf", archivePath)
	case strings.Contains(archivePath, "containerd"):
		// ✅ распаковываем bin/containerd внутрь baseDir/bin
		cmd = exec.Command("tar", "-C", filepath.Join(i.baseDir, "bin"), "--strip-components=1", "-zxf", archivePath)
	case strings.Contains(archivePath, "cni-plugins"):
		cmd = exec.Command("tar", "zxf", archivePath, "-C", "/opt/cni/bin/")
	case strings.Contains(archivePath, "crictl"):
		cmd = exec.Command("tar", "zxf", archivePath, "-C", filepath.Join(i.baseDir, "bin"))
	default:
		return fmt.Errorf("unknown archive type: %s", archivePath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extraction failed: %w, output: %s", err, output)
	}
	return nil
}
