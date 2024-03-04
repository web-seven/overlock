package environment

import (
	"os"
	"os/exec"

	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/log"
)

func K3sCluster(context string, logger *log.Logger) error {
	cmd := exec.Command("sh", "-c", "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--write-kubeconfig-mode 644' sh -")

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		logger.Error(err)
	}

	if err := cmd.Wait(); err != nil {
		logger.Error(err)
	}

	os.Setenv("KUBECONFIG", "/etc/rancher/k3s/k3s.yaml")

	configClient, err := config.GetConfigWithContext(context)
	if err != nil {
		logger.Fatal(err)
	}

	installHelmResources(configClient, logger)
	return nil
}
