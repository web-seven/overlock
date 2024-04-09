package engine

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
	ser "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

const RepoUrl = "https://charts.crossplane.io/stable"

const ChartName = "crossplane"

const ReleaseName = "kndp-crossplane"

const Namespace = "kndp-system"

var managedLabels = map[string]string{
	"app.kubernetes.io/managed-by": "kndp",
}

var extraObjects = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "{{ include \"crossplane.name\" . }}:aggregate-providers"
  labels:
    app: crossplane
    rbac.crossplane.io/aggregate-to-allowed-provider-permissions: "true"
rules:
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - create
  - update
  - patch
  - delete 
`

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

	rbac.AddToScheme(scheme.Scheme)
	extra, _, _ := ser.NewYAMLSerializer(ser.DefaultMetaFactory, scheme.Scheme,
		scheme.Scheme).Decode([]byte(extraObjects), nil, nil)

	var initParameters = map[string]any{
		"extraObjects": []any{
			extra,
		},
	}

	err = engine.Upgrade("", initParameters)
	fmt.Println(err)

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
