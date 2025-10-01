package main

import (
	"flag"
	"log"

	"github.com/dereban25/k8s-installer/internal/installer"
)

func main() {
	skipAPIWait := flag.Bool("skip-api-wait", false, "Skip waiting for API server to be ready")
	flag.Parse()

	log.SetFlags(log.LstdFlags)

	inst, err := installer.NewInstaller(*skipAPIWait)
	if err != nil {
		log.Fatalf("Failed to create installer: %v", err)
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Creating directories", inst.CreateDirectories},
		{"Downloading binaries", inst.DownloadBinaries},
		{"Generating certificates", inst.GenerateCertificates},
		{"Creating configurations", inst.CreateConfigurations},
		{"Starting containerd", inst.StartContainerd},
		{"Starting etcd", inst.StartEtcd},
		{"Starting API server", inst.StartAPIServer},
		{"Configuring kubectl", inst.ConfigureKubectl},
		{"Creating system namespaces", inst.CreateSystemNamespaces},
		{"Starting controller manager", inst.StartControllerManager},
		{"Starting scheduler", inst.StartScheduler},
		{"Starting kubelet", inst.StartKubelet},
		{"Verifying installation", inst.VerifyInstallation},
	}

	for _, step := range steps {
		log.Printf("=> %s...", step.name)
		if err := step.fn(); err != nil {
			log.Fatalf("Failed at step '%s': %v", step.name, err)
		}
		log.Printf("✓ %s completed", step.name)
	}

	log.Println("🎉 Kubernetes installation completed successfully!")
	log.Println("You can now use kubectl to interact with your cluster:")
	log.Println("  kubectl get nodes")
	log.Println("  kubectl get pods -A")
}