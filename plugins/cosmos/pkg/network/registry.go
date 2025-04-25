package network

import (
	"context"
	"regexp"
	"strings"

	storagev1beta1 "github.com/overlock-network/api/go/node/overlock/storage/v1beta1"
	"github.com/pterm/pterm"
	"github.com/web-seven/overlock/pkg/environment"
	"github.com/web-seven/overlock/pkg/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func createRegistry(engine string, ctx context.Context, logger *zap.SugaredLogger, msg storagev1beta1.MsgCreateRegistry, client *kubernetes.Clientset, config *rest.Config, clientAddress string) {
	env, err := getEnvironmentById(msg.EnvironmentId, clientAddress, logger)
	if err != nil {
		logger.Errorf("Failed to get environment: %v", err)
		return
	}
	envName := env.Metadata.Name

	tableData := pterm.TableData{{"NAME", "TYPE"}}
	tableData = environment.ListEnvironments(logger, tableData)

	envExists := false
	re := regexp.MustCompile(`-(\w+)`)
	for _, row := range tableData[1:] {
		matches := re.FindStringSubmatch(row[0])
		if len(matches) > 1 && matches[1] == envName {
			envExists = true
			break
		}
	}

	if !envExists {
		logger.Errorf("Environment %s does not exist, skipping registry creation", envName)
		return
	}

	reg := registry.NewLocal()
	reg.Name = msg.Name
	if reg.Exists(ctx, client) {
		logger.Errorf("Secret for this registry server already exists")
		return
	}

	registryContext := strings.Join([]string{engine, envName}, "-")
	reg.WithContext(registryContext)

	if err := reg.Create(ctx, config, logger); err != nil {
		logger.Errorf("Failed to create registry: %v", err)
	} else {
		logger.Infof("Successfully created registry: %s", msg.Name)
	}
}
