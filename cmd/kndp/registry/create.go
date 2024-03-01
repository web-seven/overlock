package registry

import (
	"context"
	b64 "encoding/base64"

	"github.com/charmbracelet/log"

	"github.com/kndpio/kndp/internal/registry"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type createCmd struct {
	RegistryServer string `required:"" help:"is your Private Registry FQDN."`
	Username       string `required:"" help:"is your Username."`
	Password       string `required:"" help:"is your Password."`
	Email          string `required:"" help:"is your Email."`
}

func (c *createCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *log.Logger) error {

	reg := registry.Registry{
		Config: registry.RegistryConfig{
			Auths: map[string]registry.RegistryAuth{
				c.RegistryServer: {
					Username: c.Username,
					Password: c.Password,
					Email:    c.Email,
					Auth:     b64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password)),
				},
			},
		},
	}

	if reg.Exists(ctx, client) {
		logger.Info("Secret for this registry server already exists.")
		return nil
	}
	err := reg.Create(ctx, client, config, logger)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Registry created successfully.")
	}

	return nil
}
