package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"

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
	if c.Engine == "kind" {
		environment.KindCluster(c.Context, logger, c.Name, c.HostPort, yamlTemplate)
	} else if c.Engine == "k3s" {
		err := environment.K3sCluster(c.Context, logger)
		if err != nil {
			logger.Fatal(err)
			return nil
		}
	} else {
		logger.Fatal("Kubernetes engine not supported")
		return nil
	}
	return nil
}
