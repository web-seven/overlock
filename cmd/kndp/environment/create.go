package environment

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/charmbracelet/huh"
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
	Name     string `arg:"" optional:"" help:"Name of environment."`
	HostPort int    `optional:"" short:"p" help:"Host port for mapping" default:"80"`
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	if !(len(c.Name) > 0) {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter a name for environment: ").
					Value(&c.Name),
			),
		)
		form.Run()
	}

	clusterYaml := fmt.Sprintf(yamlTemplate, c.HostPort)

	cmd := exec.Command("kind", "create", "cluster", "--name", c.Name, "--config", "-")
	cmd.Stdin = strings.NewReader(clusterYaml)

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
		line := stderrScanner.Text()
		if !strings.Contains(line, " • ") {
			fmt.Println(line)
		}
	}

	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		if !strings.Contains(line, " • ") {
			fmt.Println(line)
		}
	}
	fmt.Println("Installing crossplane ...")

	cmd.Wait()

	config := ctrl.GetConfigOrDie()
	chartName := "crossplane"
	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		return fmt.Errorf("error parsing repository URL: %v", err)
	}

	setWait := helm.InstallerModifierFn(helm.Wait())
	installer, err := helm.NewManager(config, chartName, repoURL, setWait)
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
	fmt.Println("Crossplane installation completed successfully!")
	return nil
}
