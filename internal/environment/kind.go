package environment

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/engine"
)

var yamlTemplate = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: worker
  extraMounts:
  - hostPath: ./
    containerPath: /storage
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: %d
    protocol: TCP
`

func KindEnvironment(ctx context.Context, context string, logger *log.Logger, name string, hostPort int) error {

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
		if strings.Contains(line, "ERROR") {
			logger.Fatal(line)
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
	configClient, err := config.GetConfigWithContext("kind-" + name)
	if err != nil {
		logger.Fatal(err)
	}
	return engine.InstallEngine(ctx, configClient, nil)
}
