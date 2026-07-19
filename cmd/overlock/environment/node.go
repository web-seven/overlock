package environment

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type nodeCmd struct {
	Create nodeCreateCmd `cmd:"" help:"Create a new node in an Environment"`
	Delete nodeDeleteCmd `cmd:"" help:"Delete a node from an Environment"`
}

type nodeCreateCmd struct {
	Name        string   `arg:"" required:"" help:"Name of the node."`
	Environment string   `required:"" help:"Name of the target environment (k3s cluster)."`
	Engine      string   `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s-docker"`
	Scopes      []string `optional:"" help:"Comma-separated list of node scopes (engine, workloads)."`
	Host        string   `optional:"" help:"Remote host to create the node on via SSH."`
	User        string   `optional:"" help:"SSH user for the remote host." default:"root"`
	Port        int      `optional:"" help:"SSH port for the remote host." default:"22"`
	Key         string   `optional:"" help:"Path to SSH private key." default:"~/.ssh/id_rsa"`
	Cpu         string   `optional:"" help:"CPU limit for the node container (e.g., 2, 0.5, 50%)." default:""`
	Taints      []string `optional:"" help:"Comma-separated list of node taints in key:value format (e.g., dedicated:gpu,team:ml)."`
	Mount       []string `optional:"" help:"Bind mount in host:container format (e.g., /data:/storage). Can be specified multiple times. Local nodes only."`
}

func (c *nodeCreateCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	env := environment.New(c.Engine, c.Environment).WithCpu(c.Cpu)
	return createNode(ctx, env, c.Name, c.Scopes, c.Taints, c.Host, c.User, c.Port, c.Key, c.Mount, logger)
}

// createNode creates a single node on env, equivalent to running
// "overlock env node create" for the given parameters.
func createNode(ctx context.Context, env *environment.Environment, name string, scopes, taints []string, host, user string, port int, key string, mounts []string, logger *zap.SugaredLogger) error {
	if name == "" {
		return fmt.Errorf("node configuration requires a name")
	}

	var remote *environment.SSHClient
	if host != "" {
		if user == "" {
			user = "root"
		}
		if port == 0 {
			port = 22
		}
		if key == "" {
			key = "~/.ssh/id_rsa"
		}
		var err error
		remote, err = environment.NewSSHClient(host, user, port, key)
		if err != nil {
			return fmt.Errorf("failed to create SSH client for node %q: %w", name, err)
		}
		defer remote.Close()
	}

	if len(mounts) > 0 {
		if remote != nil {
			logger.Warnf("mount is only supported for local nodes, ignoring for remote host %s", host)
		} else {
			for _, m := range mounts {
				parts := strings.SplitN(m, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid mount format %q for node %q, expected host:container", m, name)
				}
			}
			env = env.WithMounts(mounts)
		}
	}

	if err := env.CreateNode(ctx, name, scopes, taints, remote, logger); err != nil {
		return fmt.Errorf("failed to create node %q: %w", name, err)
	}
	return nil
}

type nodeDeleteCmd struct {
	Name        string   `arg:"" required:"" help:"Name of the node to delete."`
	Environment string   `required:"" help:"Name of the target environment."`
	Engine      string   `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s-docker"`
	Scopes      []string `optional:"" help:"Comma-separated list of node scopes (engine, workloads)."`
	Host        string   `optional:"" help:"Remote host where the node container runs."`
	User        string   `optional:"" help:"SSH user for the remote host." default:"root"`
	Port        int      `optional:"" help:"SSH port for the remote host." default:"22"`
	Key         string   `optional:"" help:"Path to SSH private key." default:"~/.ssh/id_rsa"`
}

func (c *nodeDeleteCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	var remote *environment.SSHClient
	if c.Host != "" {
		var err error
		remote, err = environment.NewSSHClient(c.Host, c.User, c.Port, c.Key)
		if err != nil {
			return fmt.Errorf("failed to create SSH client: %w", err)
		}
		defer remote.Close()
	}

	if err := environment.
		New(c.Engine, c.Environment).
		DeleteNode(ctx, c.Name, c.Scopes, remote, logger); err != nil {
		return fmt.Errorf("failed to delete node %q: %w", c.Name, err)
	}
	return nil
}
