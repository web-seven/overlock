package engine

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
	"k8s.io/client-go/rest"
)

const RepoUrl = "https://charts.crossplane.io/stable"

const ChartName = "crossplane"

const ReleaseName = "kndp-crossplane"

const Namespace = "kndp-system"

var managedLabels = map[string]string{
	"app.kubernetes.io/managed-by": "kndp",
}

// Get engine Helm manager
func GetEngine(configClient *rest.Config) (install.Manager, error) {
	repoURL, err := url.Parse(RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing repository URL: %v", err)
	}
	setWait := helm.InstallerModifierFn(helm.Wait())
	setNamespace := helm.InstallerModifierFn(helm.WithNamespace(Namespace))
	setUpInstall := helm.InstallerModifierFn(helm.WithUpgradeInstall(true))
	setCreateNs := helm.InstallerModifierFn(helm.WithCreateNamespace(true))

	installer, err := helm.NewManager(
		configClient,
		ChartName,
		repoURL,
		ReleaseName,
		setWait,
		setNamespace,
		setUpInstall,
		setCreateNs,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Helm manager: %v", err)
	}

	return installer, nil
}

// Install engine Helm release
func InstallEngine(configClient *rest.Config) error {
	engine, err := GetEngine(configClient)
	if err != nil {
		return err
	}
	err = engine.Upgrade("", nil)
	if err != nil {
		return err
	}
	return nil
}

// Check if engine release exists
func IsHelmReleaseFound(configClient *rest.Config) bool {

	installer, err := GetEngine(configClient)
	if err != nil {
		return false
	}
	_, err = installer.GetRelease()
	return err == nil

}

// Lables for engine system resources, mixed with provided
func ManagedLabels(m map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range managedLabels {
		merged[k] = v
	}
	for key, value := range m {
		merged[key] = value
	}
	return merged
}

// Selector for engine system resources, mixed with provided
func ManagedSelector(m map[string]string) string {
	selectors := []string{}
	for k, v := range managedLabels {
		selectors = append(selectors, k+"="+v)
	}
	for key, value := range m {
		selectors = append(selectors, key+"="+value)
	}
	return strings.Join(selectors, ",")
}
