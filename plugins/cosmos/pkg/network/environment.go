package network

import (
	"context"

	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/pkg/environment"
	"github.com/web-seven/overlock/plugins/cosmos/pkg/client"
	"go.uber.org/zap"
)

func createEnvironment(engine string, ctx context.Context, logger *zap.SugaredLogger, msg crossplanev1beta1.MsgCreateEnvironment) {

	err := environment.New(engine, msg.Metadata.Name).WithDisabledPorts(true).Create(ctx, logger)
	if err != nil {
		logger.Errorf("Error creating environment: %v", err)
		return
	}

	logger.Infof("Successfully created environment: %s", msg.Metadata.Name)
}

func getEnvironmentById(environmentId uint64, clientAddress string, logger *zap.SugaredLogger) (*crossplanev1beta1.Environment, error) {
	client, err := client.NewClient(clientAddress)
	if err != nil {
		logger.Errorf("Failed to create gRPC client: %v", err)
		return nil, err
	}

	response, err := client.ShowEnvironment(context.Background(), &crossplanev1beta1.QueryShowEnvironmentRequest{
		Id: environmentId,
	})
	if err != nil {
		logger.Errorf("Failed to get environment: %v", err)
		return nil, err
	}

	if response == nil || response.Environment == nil {
		logger.Errorf("received nil environment response")
		return nil, err
	}

	return response.Environment, nil
}
