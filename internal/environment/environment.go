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
	"github.com/kndpio/kndp/internal/ingress"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/namespace"
	"github.com/kndpio/kndp/internal/policy"
	"github.com/kndpio/kndp/internal/registry"
	"github.com/kndpio/kndp/internal/resources"
	"github.com/pterm/pterm"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/log"
)

type Environment struct {
	name      string
	engine    string
	httpPort  int
	httpsPort int
	context   string
	options   EnvironmentOptions
}

// New Environment entity
func New(engine string, name string) *Environment {
	return &Environment{
		name:   name,
		engine: engine,
	}
}

// Create environment
func (e *Environment) Create(ctx context.Context, logger *log.Logger) error {
	var err error
	if e.context == "" {
		switch e.engine {
		case "kind":
			logger.Infof("Creating environment with Kubernetes engine 'kind'")
			e.context, err = e.CreateKindEnvironment(logger)
			if err != nil {
				logger.Fatal(err)
			}
		case "k3s":
			logger.Infof("Creating environment with Kubernetes engine 'k3s'")
			e.context, err = e.CreateK3sEnvironment(logger)
			if err != nil {
				logger.Fatal(err)
			}
		case "k3d":
			logger.Infof("Creating environment with Kubernetes engine 'k3d'")
			e.context, err = e.CreateK3dEnvironment(logger)
			if err != nil {
				logger.Fatal(err)
			}
		default:
			logger.Fatalf("Kubernetes engine '%s' not supported", e.engine)
			return nil
		}
	}

	err = e.Setup(ctx, logger)
	if err != nil {
		return err
	}
	logger.Info("Environment created successfully.")
	return nil
}

// Upgrade environemnt with options or new features
func (e *Environment) Upgrade(ctx context.Context, logger *log.Logger) error {
	var err error
	if e.context == "" {
		e.context = e.GetContextName()
		if e.context == "" {
			logger.Fatalf("Kubernetes engine '%s' not supported", e.engine)
			return nil
		}
	}

	err = e.Setup(ctx, logger)
	if err != nil {
		return err
	}
	logger.Info("Environment upgraded successfully.")
	return nil
}

// Delete environment cluster
func (e *Environment) Delete(logger *log.Logger) error {
	var err error
	switch e.engine {
	case "kind":
		err = e.DeleteKindEnvironment(logger)
	case "k3s":
		err = e.DeleteK3sEnvironment(logger)
	case "k3d":
		err = e.DeleteK3dEnvironment(logger)
	default:
		logger.Fatalf("Kubernetes engine '%s' not supported", e.engine)
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

// Setup environment
func (e *Environment) Setup(ctx context.Context, logger *log.Logger) error {
	configClient, err := config.GetConfigWithContext(e.context)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debug("Start installing options")
	logger.Debugf("Ingress controller: %s", e.options.ingressController)
	err = ingress.AddIngressConroller(ctx, configClient, e.options.ingressController)
	if err != nil {
		return err
	}
	logger.Debugf("Policy controller: %s", e.options.policyController)
	err = policy.AddPolicyConroller(ctx, configClient, e.options.policyController)
	if err != nil {
		return err
	}
	logger.Debug("Done")

	logger.Debug("Preparing engine")
	installer, err := engine.GetEngine(configClient)
	if err != nil {
		return err
	}

	var params map[string]any
	release, err := installer.GetRelease()
	if err == nil {
		params = release.Config
	}

	logger.Debug("Installing engine")
	err = engine.InstallEngine(ctx, configClient, params, logger)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Debug("Done")
	return nil
}

// Get contect name specially for engine
func (e *Environment) GetContextName() string {
	var context string
	switch e.engine {
	case "kind":
		context = e.KindContextName()
	case "k3s":
		context = e.K3sContextName()
	case "k3d":
		context = e.K3dContextName()
	default:
		return ""
	}
	return context
}

// Copy Environment from source to destination contexts
func (e *Environment) CopyEnvironment(ctx context.Context, logger *log.Logger, source string, destination string) error {

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

	engine.InstallEngine(ctx, destConfig, sourceRelease.Config, logger)
	logger.Info("Engine copied successfully!")

	// Copy composities
	err = resources.CopyComposites(ctx, logger, sourceContext, destinationContext)
	if err != nil {
		return err
	}

	logger.Info("Successfully copied Environment to destination context.")

	return nil
}

// Start Environment
func (e *Environment) Start(ctx context.Context, switcher bool, logger *log.Logger) error {
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		return err
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	for _, c := range containers {
		if strings.Contains(c.Names[0], e.name) {
			containerID := c.ID
			err := dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
			if err != nil {
				logger.Errorf("Environment possible doesn't exists or failed to start %s: %v", c.ID, err)
				return err
			}
		}
	}

	if switcher {
		err := SwitchContext(e.GetContextName())
		if err != nil {
			return err
		}
	}

	logger.Infof("Environment %s started successfully.", e.name)
	return nil
}

// Stop Environment
func (e *Environment) Stop(ctx context.Context, logger *log.Logger) error {
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		return err
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range containers {
		if strings.Contains(c.Names[0], e.name) {
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

func (e *Environment) WithHttpPort(port int) *Environment {
	e.httpPort = port
	return e
}

func (e *Environment) WithHttpsPort(port int) *Environment {
	e.httpsPort = port
	return e
}

func (e *Environment) WithContext(context string) *Environment {
	e.context = context
	return e
}

func (e *Environment) WithIngressController(icname string) *Environment {
	e.options.ingressController = icname
	return e
}

func (e *Environment) WithPolicyController(pcname string) *Environment {
	e.options.policyController = pcname
	return e
}

func SwitchContext(name string) (err error) {
	newConfig := clientcmd.GetConfigFromFileOrDie(clientcmd.RecommendedHomeFile)
	newConfig.CurrentContext = name
	err = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), *newConfig, true)
	return
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
