package environment

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/pterm/pterm"
)

type deleteCmd struct {
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", c.Name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cmd.Start()

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		fmt.Println(stderrScanner.Text(), "1")
	}
	return nil
}
