package environment

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

func CreateK3sEnvironment(ctx context.Context, logger *log.Logger, name string) (string, error) {
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

	return K3sContextName(name), nil
}

func DeleteK3sEnvironment(name string, logger *log.Logger) error {
	return nil
}

func K3sContextName(name string) string {
	return name
}
