package environment

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

func KindEnvironment(context string, logger *log.Logger, name string, hostPort int, yamlTemplate string) {
	if context == "" {
		if !(len(name) > 0) {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Enter a name for environment: ").
						Value(&name),
				),
			)
			form.Run()
		}

		clusterYaml := fmt.Sprintf(yamlTemplate, hostPort)

		cmd := exec.Command("kind", "create", "cluster", "--name", name, "--config", "-")
		cmd.Stdin = strings.NewReader(clusterYaml)

		stderr, err := cmd.StderrPipe()
		if err != nil {
			logger.Errorf("error creating StderrPipe: %v", err)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Errorf("error creating StdoutPipe: %v", err)
		}

		cmd.Start()

		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			if !strings.Contains(line, " • ") {
				logger.Print(line)
			}
		}

		stdoutScanner := bufio.NewScanner(stdout)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			if !strings.Contains(line, " • ") {
				logger.Print(line)
			}
		}

		cmd.Wait()
		configClient, err := ctrl.GetConfig()
		if err != nil {
			logger.Fatal(err)
		}
		installEngine(configClient, logger)
	} else {
		configClient, err := config.GetConfigWithContext(context)
		if err != nil {
			logger.Fatal(err)
		}
		installEngine(configClient, logger)
	}
}
