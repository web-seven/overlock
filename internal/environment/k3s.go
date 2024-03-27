package environment

import (
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/engine"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func K3sEnvironment(context string, logger *log.Logger, name string) error {
	cmd := exec.Command("sudo", "k3s", "server",
		"--write-kubeconfig-mode", "0644",
		"--node-name", name,
		"--cluster-init")

	// Set the KUBECONFIG environment variable for the k3s process only
	if os.Getenv("KUBECONFIG") == "" {
		os.Setenv("KUBECONFIG", "/etc/rancher/k3s/k3s.yaml")
	}

	err := cmd.Start()
	if err != nil {
		panic(err)
	}

	// Wait for some time
	time.Sleep(10 * time.Second)

	logger.Info("k3s server started successfully")

	configClient, err := config.GetConfig()
	if err != nil {
		logger.Fatal(err)
	}

	engine.InstallEngine(configClient)
	return nil
}
