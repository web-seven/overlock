package environment

import (
	"context"

	"github.com/pterm/pterm"
	createcluster "sigs.k8s.io/kind/pkg/cmd/kind/create/cluster"
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
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	createcluster.NewCommand()
	return nil
}
