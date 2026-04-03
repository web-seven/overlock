package registry

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/registry"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	fieldErrMsg = "Field validation for '%s' failed on the '%s' tag"
)

type createCmd struct {
	RegistryServer string   `help:"is your Private Registry FQDN."`
	Username       string   `help:"is your Username."`
	Password       string   `help:"is your Password."`
	Email          string   `help:"is your Email."`
	Default        bool     `help:"Set registry as default."`
	Local          bool     `help:"Create local registry."`
	Context        string   `short:"c" help:"Kubernetes context where registry will be created."`
	Label          []string `short:"l" help:"Label to attach to the registry secret in key:value format. Can be specified multiple times."`
}

func (c *createCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *zap.SugaredLogger) error {
	reg := registry.New(c.RegistryServer, c.Username, c.Password, c.Email)
	if c.Local {
		reg = registry.NewLocal()
	}
	reg.SetDefault(c.Default)
	reg.SetLocal(c.Local)
	reg.WithContext(c.Context)

	if len(c.Label) > 0 {
		labels := make(map[string]string, len(c.Label))
		for _, l := range c.Label {
			parts := strings.SplitN(l, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid label format %q: expected key:value", l)
			}
			labels[parts[0]] = parts[1]
		}
		reg.WithLabels(labels)
	}

	if reg.Exists(ctx, client) {
		logger.Infof("Registry '%s' already exists. Using existing registry.", reg.Name)
		return nil
	}

	err := reg.Validate(ctx, client, logger)
	if err != nil {
		return err
	}

	err = reg.Create(ctx, config, logger)
	if err != nil {
		return err
	}
	logger.Info("Registry created successfully.")
	return nil
}
