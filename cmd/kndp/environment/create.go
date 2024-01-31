package environment

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
)

var cluserYaml = `
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
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP  
`

type createCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter) error {

	cmd := exec.Command("kind", "create", "cluster", "--name", c.Name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
