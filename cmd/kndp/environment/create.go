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

// AfterApply sets default values in command after assignment and validation.
// func (c *createCmd) AfterApply(insCtx *install.Context) error {

// 	repo, err := url.Parse("https://charts.crossplane.io/stable")
// 	if err != nil {
// 		fmt.Println("Error parsing repository URL:", err)
// 		os.Exit(1)
// 	}
// 	chartName := "crossplane"
// 	mgr, err := helm.NewManager(insCtx.Kubeconfig,
// 		chartName,
// 		repo,
// 		helm.WithNamespace(insCtx.Namespace))
// 	if err != nil {
// 		return err
// 	}
// 	c.mgr = mgr
// 	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
// 	if err != nil {
// 		return err
// 	}
// 	c.kClient = client
// 	base := map[string]any{}
// 	if c.File != nil {
// 		defer c.File.Close() //nolint:errcheck,gosec
// 		b, err := io.ReadAll(c.File)
// 		if err != nil {
// 			return errors.Wrap(err, errReadParametersFile)
// 		}
// 		if err := yaml.Unmarshal(b, &base); err != nil {
// 			return errors.Wrap(err, errReadParametersFile)
// 		}
// 		if err := c.File.Close(); err != nil {
// 			return errors.Wrap(err, errReadParametersFile)
// 		}
// 	}
// 	c.parser = helm.NewParser(base, c.Set)
// 	return nil
// }

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

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		fmt.Println(scanner.Text(), "1")
	}

	scanner2 := bufio.NewScanner(stdout)
	for scanner2.Scan() {
		fmt.Println(scanner.Text(), "2")
	}

	cmd.Wait()

	// namespace := &corev1.Namespace{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: c.Name,
	// 	},
	// }

	// _, err = c.kClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	// if err != nil && !errors.IsAlreadyExists(err) {
	// 	return fmt.Errorf("error creating namespace: %v", err)
	// }

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
