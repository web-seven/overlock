package environment

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os/exec"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
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

type createCmd struct {
	Name     string `arg:"" required:"" help:"Name of environment."`
	HostPort int    `optional:"" short:"p" help:"Host port for mapping" default:"80"`
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter) error {

	clusterYaml := fmt.Sprintf(yamlTemplate, c.HostPort)

	cmd := exec.Command("kind", "create", "cluster", "--name", c.Name, "--config", "-")
	fmt.Println(clusterYaml)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating StderrPipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}

	cmd.Start()

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		fmt.Println(stderrScanner.Text(), "1")
	}

	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		fmt.Println(stderrScanner.Text(), "2")
	}

	cmd.Wait()

	config := ctrl.GetConfigOrDie()
	chartName := "crossplane"
	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		return fmt.Errorf("error parsing repository URL: %v", err)
	}

	installer, err := helm.NewManager(config, chartName, repoURL)
	if err != nil {
		return fmt.Errorf("error creating Helm manager: %v", err)
	}

	err = installer.Install("", nil)
	if err != nil {
		return fmt.Errorf("error installing Helm chart: %v", err)
	}

	_, err = installer.GetCurrentVersion()
	if err != nil {
		return err
	}
	return nil
}
