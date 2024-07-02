package environment

import (
	"bufio"
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
  - containerPort: 443
    hostPort: %d
    protocol: TCP
`

func (e *Environment) CreateKindEnvironment(logger *log.Logger) (string, error) {

	clusterYaml := fmt.Sprintf(yamlTemplate, e.httpPort, e.httpsPort)

	cmd := exec.Command("kind", "create", "cluster", "--name", e.name, "--config", "-")
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
	return e.KindContextName(), nil
}

func (e *Environment) DeleteKindEnvironment(logger *log.Logger) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", e.name)
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

func (e *Environment) KindContextName() string {
	return "kind-" + e.name
}
