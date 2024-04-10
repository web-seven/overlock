package environment

import (
	"context"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/engine"
	ctrl "sigs.k8s.io/controller-runtime"
)

func K3dEnvironment(ctx context.Context, context string, logger *log.Logger, name string) error {

	cmd := exec.Command("k3d", "cluster", "create", name)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		logger.Fatalf("Error creating k3d cluster: %v", err)
	}

	configClient, err := ctrl.GetConfig()
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("k3d cluster created successfully")

	engine.InstallEngine(ctx, configClient)
	return nil
}
