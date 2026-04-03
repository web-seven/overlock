package environment

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (e *Environment) CreateK3dEnvironment(logger *zap.SugaredLogger) (string, error) {
	// Check if cluster already exists
	checkCmd := exec.Command("k3d", "cluster", "list", "-o", "json")
	output, err := checkCmd.Output()
	if err == nil && strings.Contains(string(output), `"`+e.name+`"`) {
		logger.Infof("Environment '%s' already exists. Using existing environment.", e.name)
		return e.K3dContextName(), nil
	}

	args := []string{
		"cluster", "create", e.name,
	}

	for _, m := range e.mounts {
		args = append(args, "-v", m)
	}

	cmd := exec.Command("k3d", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "error creating k3d cluster")
	}

	logger.Info("k3d cluster created successfully")
	return e.K3dContextName(), nil
}

func (e *Environment) DeleteK3dEnvironment(logger *zap.SugaredLogger) error {
	cmd := exec.Command("k3d", "cluster", "delete", e.name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start k3d command: %w", err)
	}

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		logger.Info(stderrScanner.Text())
	}
	return nil
}

func (e *Environment) K3dContextName() string {
	return "k3d-" + e.name
}
