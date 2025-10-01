package main

import (
	"flag"
	"log"

	"github.com/dereban25/k8s-installer/internal/installer"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var (
		k8sVersion       = flag.String("k8s-version", "v1.30.0", "Kubernetes version")
		skipDownload     = flag.Bool("skip-download", false, "Skip downloading binaries")
		skipVerify       = flag.Bool("skip-verify", false, "Skip verification")
		skipAPIWait      = flag.Bool("skip-api-wait", false, "Skip waiting for API server (faster but less safe)")
		continueOnError  = flag.Bool("continue-on-error", false, "Continue installation even if non-critical steps fail")
		verbose          = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	}

	inst, err := installer.New(&installer.Config{
		K8sVersion:      *k8sVersion,
		SkipDownload:    *skipDownload,
		SkipVerify:      *skipVerify,
		SkipAPIWait:     *skipAPIWait,
		ContinueOnError: *continueOnError,
		Verbose:         *verbose,
	})
	if err != nil {
		log.Fatalf("Failed to create installer: %v", err)
	}

	if err := inst.Run(); err != nil {
		log.Fatalf("Installation failed: %v", err)
	}

	log.Println("🎉 Kubernetes installation completed successfully!")
}