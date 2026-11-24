package environment

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type upgradeCmd struct {
	Name                      string `arg:"" required:"" help:"Environment name where engine will be upgraded."`
	Engine                    string `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	Context                   string `optional:"" short:"c" help:"Kubernetes context where Environment will be upgraded."`
	CreateAdminServiceAccount bool   `optional:"" help:"Create admin service account with cluster-admin privileges."`
	AdminServiceAccountName   string `optional:"" help:"Name for the admin service account. Only relevant when create-admin-service-account is enabled. Defaults to 'overlock-admin' if not specified."`
}

func (c *upgradeCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Name).
		WithContext(c.Context).
		WithAdminServiceAccount(c.CreateAdminServiceAccount, c.AdminServiceAccountName).
		Upgrade(ctx, logger)
}
