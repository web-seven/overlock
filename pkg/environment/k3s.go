package environment

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (e *Environment) CreateK3sEnvironment(logger *zap.SugaredLogger) (string, error) {
	// Check if k3s is already running with this node name
	checkCmd := exec.Command("pgrep", "-f", "k3s.*--node-name.*"+e.name)
	if err := checkCmd.Run(); err == nil {
		logger.Infof("Environment '%s' already exists. Using existing environment.", e.name)
		return e.K3sContextName(), nil
	}

	args := []string{
		"k3s", "server",
		"--write-kubeconfig-mode", "0644",
		"--node-name", e.name,
		"--cluster-init",
		"--disable=traefik",
	}

	if len(e.mounts) > 0 {
		args = append(args, "--data-dir", strings.SplitN(e.mounts[0], ":", 2)[0])
	}

	cmd := exec.Command("sudo", args...)

	// Set the KUBECONFIG environment variable for the k3s process only
	if os.Getenv("KUBECONFIG") == "" {
		if err := os.Setenv("KUBECONFIG", "/etc/rancher/k3s/k3s.yaml"); err != nil {
			return "", fmt.Errorf("failed to set KUBECONFIG: %w", err)
		}
	}

	err := cmd.Start()
	if err != nil {
		panic(err)
	}

	// Wait for some time
	time.Sleep(10 * time.Second)

	logger.Info("k3s server started successfully")

	return e.K3sContextName(), nil
}

func (e *Environment) DeleteK3sEnvironment(logger *zap.SugaredLogger) error {
	return nil
}

func (e *Environment) K3sContextName() string {
	return e.name
}
