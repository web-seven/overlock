package environment

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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
	Context  string `optional:"" short:"c" help:"Kubernetes context where Environment will be created."`
}

func installHelmResources(configClient *rest.Config) error {
	fmt.Println("Installing crossplane ...")

	chartName := "crossplane"
	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		return fmt.Errorf("error parsing repository URL: %v", err)
	}

	setWait := helm.InstallerModifierFn(helm.Wait())
	installer, err := helm.NewManager(configClient, chartName, repoURL, setWait)
	if err != nil {
		return fmt.Errorf("error creating Helm manager: %v", err)
	}

	err = installer.Install("", nil)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Crossplane installation completed successfully!")
	return nil
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	if c.Context == "" {

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

		cmd.Wait()
		configClient, err := ctrl.GetConfig()
		if err != nil {
			fmt.Println(err)
		}
		installHelmResources(configClient)
	} else {
		configClient, err := config.GetConfigWithContext(c.Context)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		installHelmResources(configClient)
	}
	return nil
}
