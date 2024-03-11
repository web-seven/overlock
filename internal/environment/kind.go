package environment

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/charmbracelet/log"
)

func KindEnvironment(context string, logger *log.Logger, name string, hostPort int, yamlTemplate string) {

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
	InstallEngine(configClient, logger)
}
