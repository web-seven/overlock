package environment

import (
	"bufio"
	"context"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
)

func CreateK3dEnvironment(ctx context.Context, logger *log.Logger, name string) (string, error) {

	cmd := exec.Command("k3d", "cluster", "create", name)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		logger.Fatalf("Error creating k3d cluster: %v", err)
	}

	logger.Info("k3d cluster created successfully")
	return K3dContextName(name), nil
}

func DeleteK3dEnvironment(name string, logger *log.Logger) error {
	cmd := exec.Command("k3d", "cluster", "delete", name)
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
func K3dContextName(name string) string {
	return "k3d-" + name
}
