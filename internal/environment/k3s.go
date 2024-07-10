package environment

import (
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

func (e *Environment) CreateK3sEnvironment(logger *log.Logger) (string, error) {

	args := []string{
		"k3s", "server",
		"--write-kubeconfig-mode", "0644",
		"--node-name", e.name,
		"--cluster-init",
	}

	if e.mountPath != "" {
		args = append(args, "--data-dir", e.mountPath)
	}

	cmd := exec.Command("sudo", args...)

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

	return e.K3sContextName(), nil
}

func (e *Environment) DeleteK3sEnvironment(logger *log.Logger) error {
	return nil
}

func (e *Environment) K3sContextName() string {
	return e.name
}
