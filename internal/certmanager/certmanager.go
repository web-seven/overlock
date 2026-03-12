package certmanager

import (
	"context"
	"net/url"

	"github.com/web-seven/overlock/internal/install/helm"
	"github.com/web-seven/overlock/internal/namespace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	certManagerChartName    = "cert-manager"
	certManagerChartVersion = "v1.16.2"
	certManagerReleaseName  = "cert-manager"
	certManagerRepoUrl      = "https://charts.jetstack.io"
	certManagerNamespace    = "cert-manager"

	clusterIssuerName  = "overlock-selfsigned"
	registryCertName   = "registry-tls"
	registrySecretName = "registry-tls"
)

var (
	certManagerValues = map[string]interface{}{
		"crds": map[string]interface{}{
			"enabled": true,
		},
	}
)

// InstallCertManager installs cert-manager via Helm if not already installed
func InstallCertManager(ctx context.Context, config *rest.Config, extraParams map[string]any) error {
	repoURL, err := url.Parse(certManagerRepoUrl)
	if err != nil {
		return err
	}

	manager, err := helm.NewManager(config, certManagerChartName, repoURL, certManagerReleaseName,
		helm.InstallerModifierFn(helm.Wait()),
		helm.InstallerModifierFn(helm.WithNamespace(certManagerNamespace)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(true)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(true)),
	)
	if err != nil {
		return err
	}

	release, _ := manager.GetRelease()
	if release != nil {
		return nil
	}

	values := make(map[string]interface{}, len(certManagerValues))
	for k, v := range certManagerValues {
		values[k] = v
	}
	for k, v := range extraParams {
		values[k] = v
	}

	err = manager.Upgrade(certManagerChartVersion, values)
	if err != nil {
		return err
	}

	return nil
}

// CreateSelfSignedIssuer creates a self-signed ClusterIssuer for overlock
func CreateSelfSignedIssuer(ctx context.Context, config *rest.Config) error {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	issuer := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "ClusterIssuer",
			"metadata": map[string]interface{}{
				"name": clusterIssuerName,
			},
			"spec": map[string]interface{}{
				"selfSigned": map[string]interface{}{},
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "clusterissuers",
	}

	// Check if issuer already exists
	_, err = dynamicClient.Resource(gvr).Get(ctx, clusterIssuerName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	_, err = dynamicClient.Resource(gvr).Create(ctx, issuer, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// CreateRegistryCertificate creates a Certificate for the registry service
func CreateRegistryCertificate(ctx context.Context, config *rest.Config) error {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	cert := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      registryCertName,
				"namespace": namespace.Namespace,
			},
			"spec": map[string]interface{}{
				"secretName": registrySecretName,
				"duration":   "8760h",
				"renewBefore": "720h",
				"dnsNames": []interface{}{
					"registry",
					"registry." + namespace.Namespace,
					"registry." + namespace.Namespace + ".svc",
					"registry." + namespace.Namespace + ".svc.cluster.local",
					"localhost",
				},
				"ipAddresses": []interface{}{
					"127.0.0.1",
				},
				"issuerRef": map[string]interface{}{
					"name": clusterIssuerName,
					"kind": "ClusterIssuer",
				},
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}

	// Check if certificate already exists
	_, err = dynamicClient.Resource(gvr).Namespace(namespace.Namespace).Get(ctx, registryCertName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	_, err = dynamicClient.Resource(gvr).Namespace(namespace.Namespace).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetRegistrySecretName returns the name of the TLS secret for the registry
func GetRegistrySecretName() string {
	return registrySecretName
}
