package network

import (
	"context"

	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/pkg/environment"
	"go.uber.org/zap"
)

func createEnvironment(engine string, ctx context.Context, logger *zap.SugaredLogger, msg crossplanev1beta1.Environment) {

	err := environment.New(engine, msg.Metadata.Name).WithDisabledPorts(true).Create(ctx, logger)
	if err != nil {
		logger.Errorf("Error creating environment: %v", err)
		return
	}

	logger.Infof("Successfully created environment: %s", msg.Metadata.Name)
}
