package environment

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
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

func CreateKindEnvironment(ctx context.Context, logger *log.Logger, name string, hostPort int) (string, error) {

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
	return KindContextName(name), nil
}

func DeleteKindEnvironment(name string, logger *log.Logger) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", name)
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

func KindContextName(name string) string {
	return "kind-" + name
}
