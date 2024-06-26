package environment

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/kndpio/kndp/internal/engine"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/namespace"
	"github.com/kndpio/kndp/internal/registry"
	"github.com/kndpio/kndp/internal/resources"
	"github.com/pterm/pterm"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

// Create environment
func Create(ctx context.Context, context string, engineName string, name string, port int, logger *log.Logger) error {
	if context == "" {
		if !(len(name) > 0) {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Enter a name for environment: ").
						Value(&name),
				),
			)
			form.Run()
		}
		switch engineName {
		case "kind":
			logger.Infof("Creating environment with Kubernetes engine 'kind'")
			err := KindEnvironment(ctx, context, logger, name, port)
			if err != nil {
				logger.Fatal(err)
			}
		case "k3s":
			logger.Infof("Creating environment with Kubernetes engine 'k3s'")
			err := K3sEnvironment(ctx, context, logger, name)
			if err != nil {
				logger.Fatal(err)
			}
		case "k3d":
			logger.Infof("Creating environment with Kubernetes engine 'k3d'")
			err := K3dEnvironment(ctx, context, logger, name)
			if err != nil {
				logger.Fatal(err)
			}
		default:
			logger.Fatalf("Kubernetes engine '%s' not supported", engineName)
		}

	} else {
		configClient, err := config.GetConfigWithContext(context)
		if err != nil {
			logger.Fatal(err)
		}

		return engine.InstallEngine(ctx, configClient, nil)
	}
	logger.Info("Environment created successfully.")
	return nil
}

// Copy Environment from source to destination contexts
func CopyEnvironment(ctx context.Context, logger *log.Logger, source string, destination string) error {

	// Create a REST clients
	sourceConfig, err := kube.Config(source)
	if err != nil {
		return err
	}

	destConfig, err := kube.Config(destination)
	if err != nil {
		return err
	}

	// Create a Kubernetes contexts
	sourceContext, err := kube.ConfigContext(ctx, sourceConfig)
	if err != nil {
		return err
	}
	destinationContext, err := kube.ConfigContext(ctx, destConfig)
	if err != nil {
		return err
	}

	// Create namespace on destination
	err = namespace.CreateNamespace(ctx, destConfig)
	if err != nil {
		return err
	}

	// Copy registries
	err = registry.CopyRegistries(ctx, logger, sourceConfig, destConfig)
	if err != nil {
		return err
	}

	// Copy engine
	logger.Info("Start copy engine...")
	sourceEngine, err := engine.GetEngine(sourceConfig)
	if err != nil {
		return err
	}

	sourceRelease, err := sourceEngine.GetRelease()
	if err != nil {
		return err
	}

	engine.InstallEngine(ctx, destConfig, sourceRelease.Config)
	logger.Info("Engine copied successfully!")

	// Copy composities
	err = resources.CopyComposites(ctx, logger, sourceContext, destinationContext)
	if err != nil {
		return err
	}

	logger.Info("Successfully copied Environment to destination context.")

	return nil
}

// List Environments in available contexts
func ListEnvironments(logger *log.Logger, tableData pterm.TableData) pterm.TableData {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	configFile := clientcmd.GetConfigFromFileOrDie(kubeconfig)

	for name := range configFile.Contexts {
		configClient, err := config.GetConfigWithContext(name)
		if err != nil {
			logger.Fatal(err)
		}
		if engine.IsHelmReleaseFound(configClient) {
			types := regexp.MustCompile(`(\w+)`).FindStringSubmatch(name)
			tableData = append(tableData, []string{name, strings.ToUpper(types[0])})
		}
	}
	return tableData
}

// Stop Environment
func Stop(ctx context.Context, name string, logger *log.Logger) error {
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		return err
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range containers {
		if strings.Contains(c.Names[0], name) {
			containerID := c.ID
			err := dockerClient.ContainerStop(ctx, containerID, container.StopOptions{})
			if err != nil {
				return err
			}
		}
	}
	logger.Info("Environment stopped successfully.")
	return nil
}

// Start Environment
func Start(ctx context.Context, name string, switcher bool, logger *log.Logger) error {
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		return err
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range containers {
		if strings.Contains(c.Names[0], name) {
			containerID := c.ID
			err := dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
			if err != nil {
				logger.Errorf("Failed to start environment %s: %v", c.ID, err)
				return err
			}
			if switcher {
				SwitchContext("kind-" + name)
			}
			logger.Infof("Environment %s started successfully.", name)
			return nil
		}
	}
	logger.Errorf("Environment %s does not exist.", name)
	return nil
}

func SwitchContext(name string) (err error) {
	newConfig := clientcmd.GetConfigFromFileOrDie(clientcmd.RecommendedHomeFile)
	newConfig.CurrentContext = name
	err = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), *newConfig, true)
	return
}
