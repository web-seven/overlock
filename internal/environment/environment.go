package environment

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kndpio/kndp/internal/configuration"
	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/resources"
	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/charmbracelet/log"
)

const ReleaseName = "kndp-crossplane"

func MoveKndpResources(ctx context.Context, logger *log.Logger, source string, destination string) error {

	// Create a Kubernetes client
	sourceContext := kube.Context(ctx, logger, source)
	destinationContext := kube.Context(ctx, logger, destination)

	//Apply configurations
	paramsConfiguration := kube.ResourceParams{
		Dynamic:    sourceContext,
		Ctx:        ctx,
		Group:      "pkg.crossplane.io",
		Version:    "v1",
		Resource:   "configurations",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	}
	configurations, err := configuration.GetConfiguration(ctx, logger, sourceContext, paramsConfiguration)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Check configuration health status and move configurations to destination cluster
	err = configuration.MoveConfigurations(ctx, logger, destinationContext, configurations, paramsConfiguration)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Get composite resources from XRDs definition and apply them
	XRDs, err := kube.GetKubeResources(kube.ResourceParams{
		Dynamic:    sourceContext,
		Ctx:        ctx,
		Group:      "apiextensions.crossplane.io",
		Version:    "v1",
		Resource:   "compositeresourcedefinitions",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	})
	if err != nil {
		logger.Fatal(err)
		return nil
	}
	err = resources.MoveCompositeResources(ctx, logger, sourceContext, destinationContext, XRDs)

	//Delete source cluster after all resources are successfully created in destination cluster
	if err == nil {
		cmd := exec.Command("kind", "delete", "cluster", "--name", strings.TrimPrefix(source, "kind-"))
		cmd.Run()
		logger.Info("Successfully moved Kubernetes resources to the destination cluster.")
	}

	return nil
}

func InstallEngine(configClient *rest.Config, logger *log.Logger) error {
	logger.Info("Installing crossplane ...")

	chartName := "crossplane"
	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		logger.Errorf("error parsing repository URL: %v", err)
	}
	setWait := helm.InstallerModifierFn(helm.Wait())
	installer, err := helm.NewManager(configClient, chartName, repoURL, ReleaseName, setWait)
	if err != nil {
		logger.Errorf("error creating Helm manager: %v", err)
	}

	err = installer.Install("", nil)
	if err != nil {
		logger.Error(err)
	}

	logger.Info("Crossplane installation completed successfully!")
	return nil
}

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
		if IsHelmReleaseFound(configClient, logger, ReleaseName) {
			types := regexp.MustCompile(`(\w+)`).FindStringSubmatch(name)
			tableData = append(tableData, []string{name, strings.ToUpper(types[0])})
		}
	}
	return tableData
}

func IsHelmReleaseFound(configClient *rest.Config, logger *log.Logger, chartName string) bool {

	installer, err := helm.NewManager(configClient, chartName, &url.URL{}, ReleaseName)
	if err != nil {
		return false
	}
	_, err = installer.GetRelease()
	return err == nil

}
