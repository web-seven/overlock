package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/huh"
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

type createCmd struct {
	Name     string `arg:"" optional:"" help:"Name of environment."`
	HostPort int    `optional:"" short:"p" help:"Host port for mapping" default:"80"`
	Context  string `optional:"" short:"c" help:"Kubernetes context where Environment will be created."`
	Engine   string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
}

func (c *createCmd) Run(ctx context.Context, logger *log.Logger) error {
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
		switch c.Engine {
		case "kind":
			logger.Infof("Creating environment with Kubernetes engine 'kind'")
			environment.KindEnvironment(c.Context, logger, c.Name, c.HostPort, yamlTemplate)
		case "k3s":
			logger.Infof("Creating environment with Kubernetes engine 'k3s'")
			err := environment.K3sEnvironment(c.Context, logger, c.Name)
			if err != nil {
				logger.Fatal(err)
			}
		case "k3d":
			logger.Infof("Creating environment with Kubernetes engine 'k3d'")
			err := environment.K3dEnvironment(c.Context, logger, c.Name)
			if err != nil {
				logger.Fatal(err)
			}
		default:
			logger.Fatalf("Kubernetes engine '%s' not supported", c.Engine)
		}

	} else {
		configClient, err := config.GetConfigWithContext(c.Context)
		if err != nil {
			logger.Fatal(err)
		}
		environment.InstallEngine(configClient, logger)
	}

	return nil

}
