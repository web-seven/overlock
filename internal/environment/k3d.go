package environment

import (
	"bufio"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
)

func (e *Environment) CreateK3dEnvironment(logger *log.Logger) (string, error) {

	cmd := exec.Command("k3d", "cluster", "create", e.name)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		logger.Fatalf("Error creating k3d cluster: %v", err)
	}

	logger.Info("k3d cluster created successfully")
	return e.K3dContextName(), nil
}

func (e *Environment) DeleteK3dEnvironment(logger *log.Logger) error {
	cmd := exec.Command("k3d", "cluster", "delete", e.name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Start()

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		logger.Print(stderrScanner.Text())
	}
	return nil
}

func (e *Environment) K3dContextName() string {
	return "k3d-" + e.name
}
