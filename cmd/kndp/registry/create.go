package registry

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/go-playground/validator/v10"

	"github.com/kndpio/kndp/internal/registry"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	fieldErrMsg = "Field validation for '%s' failed on the '%s' tag"
)

type createCmd struct {
	RegistryServer string `required:"" help:"is your Private Registry FQDN."`
	Username       string `required:"" help:"is your Username."`
	Password       string `required:"" help:"is your Password."`
	Email          string `required:"" help:"is your Email."`
	Default        bool   `help:"Set registry as default."`
	Local          bool   `help:"Create local registry."`
}

func (c *createCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *log.Logger) error {

	reg := registry.New(c.RegistryServer, c.Username, c.Password, c.Email)
	reg.SetDefault(c.Default)
	reg.SetLocal(c.Local)
	verr := reg.Validate()
	if verr != nil {
		errs := verr.(validator.ValidationErrors)
		for _, err := range errs {
			logger.Errorf(fieldErrMsg, err.Field(), err.Tag())
		}
		return nil
	}

	if reg.Exists(ctx, client) {
		logger.Info("Secret for this registry server already exists.")
		return nil
	}
	err := reg.Create(ctx, config, logger)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Registry created successfully.")
	}

	return nil
}
