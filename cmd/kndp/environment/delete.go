package environment

import (
	"bufio"
	"context"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
)

type deleteCmd struct {
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *deleteCmd) Run(ctx context.Context, logger *log.Logger) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", c.Name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error(err)
		os.Exit(1)
	}
	cmd.Start()

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		logger.Print(stderrScanner.Text())
	}
	return nil
}
