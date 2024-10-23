package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/web-seven/overlock/internal/install"
	"github.com/web-seven/overlock/internal/install/helm"
	"github.com/web-seven/overlock/internal/namespace"
	"k8s.io/client-go/rest"

	"go.uber.org/zap"
)

const (
	RepoUrl             = "https://charts.crossplane.io/stable"
	ChartName           = "crossplane"
	ReleaseName         = "overlock-crossplane"
	Version             = "1.17.1"
	trueVal             = "true"
	errParsePackageName = "package name is not valid"
)

var (
	managedLabels = map[string]string{
		"app.kubernetes.io/managed-by": "overlock",
	}
	initParameters = map[string]any{
		"provider": map[string]any{
			"packages": []string{},
		},
		"configuration": map[string]any{
			"packages": []string{},
		},
		"args": []string{},
	}
)

// Get engine Helm manager
func GetEngine(configClient *rest.Config) (install.Manager, error) {
	repoURL, err := url.Parse(RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing repository URL: %v", err)
	}
	setWait := helm.InstallerModifierFn(helm.Wait())
	setNamespace := helm.InstallerModifierFn(helm.WithNamespace(namespace.Namespace))
	setUpInstall := helm.InstallerModifierFn(helm.WithUpgradeInstall(true))
	setCreateNs := helm.InstallerModifierFn(helm.WithCreateNamespace(true))
	setReuseValues := helm.InstallerModifierFn(helm.WithReuseValues(true))

	installer, err := helm.NewManager(
		configClient,
		ChartName,
		repoURL,
		ReleaseName,
		setWait,
		setNamespace,
		setUpInstall,
		setCreateNs,
		setReuseValues,
	)

	if err != nil {
		return nil, fmt.Errorf("error creating Helm manager: %v", err)
	}

	return installer, nil
}

// Install engine Helm release
func InstallEngine(ctx context.Context, configClient *rest.Config, params map[string]any, logger *zap.SugaredLogger) error {
	engine, err := GetEngine(configClient)
	if err != nil {
		return err
	}

	if params == nil {
		params = initParameters
	}
	logger.Debug("Upgrade Crossplane release")
	return engine.Upgrade(Version, params)
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

func BuildPack(pack v1.Package, img string, pkgMap map[string]string) error {
	ref, err := name.ParseReference(img, name.WithDefaultRegistry(""))
	if err != nil {
		return errors.Wrap(err, errParsePackageName)
	}
	objName := ToDNSLabel(ref.Context().RepositoryStr())
	if existing, ok := pkgMap[ref.Context().RepositoryStr()]; ok {
		objName = existing
	}
	pack.SetName(objName)
	pack.SetSource(ref.String())
	return nil
}

// ToDNSLabel converts the string to a valid DNS label.
func ToDNSLabel(s string) string {
	var cut strings.Builder
	for i := range s {
		b := s[i]
		if ('a' <= b && b <= 'z') || ('0' <= b && b <= '9') {
			cut.WriteByte(b)
		}
		if (b == '.' || b == '/' || b == ':' || b == '-') && (i != 0 && i != 62 && i != len(s)-1) {
			cut.WriteByte('-')
		}
		if i == 62 {
			break
		}
	}
	return strings.Trim(cut.String(), "-")
}
