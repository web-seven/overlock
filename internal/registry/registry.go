package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/kndpio/kndp/internal/engine"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/namespace"
	"github.com/kndpio/kndp/internal/policy"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	kv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	cfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	RegistryServerLabel = "kndp-registry-server-url"
	DefaultRemoteDomain = "xpkg.upbound.io"
	LocalServiceName    = "registry"
	DefaultLocalDomain  = LocalServiceName + "." + namespace.Namespace + ".svc.cluster.local"
	AuthConfigLabel     = "kndp-registry-auth-config"
)

type RegistryAuth struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Server   string `json:"server" validate:"required,http_url"`
}

type RegistryConfig struct {
	Auths map[string]RegistryAuth `json:"auths"`
}

type Registry struct {
	Config  RegistryConfig
	Default bool
	Local   bool
	Context string
	corev1.Secret
}

// Return regestires from requested context
func Registries(ctx context.Context, client *kubernetes.Clientset) ([]*Registry, error) {
	secrets, err := secretClient(client).
		List(ctx, metav1.ListOptions{LabelSelector: AuthConfigLabel + "=true"})
	if err != nil {
		return nil, err
	}
	registries := []*Registry{}
	for _, secret := range secrets.Items {
		registry := Registry{}
		registries = append(registries, registry.FromSecret(secret))
	}
	return registries, nil
}

// Creates new Registry by required parameters
func New(server string, password string, username string, email string) Registry {
	registry := Registry{
		Default: false,
		Config: RegistryConfig{
			Auths: map[string]RegistryAuth{
				"server": {
					Password: password,
					Username: username,
					Email:    email,
					Server:   server,
				},
			},
		},
	}
	registry.Annotations = map[string]string{
		RegistryServerLabel: server,
	}
	return registry
}

// Validate data in Registry object
func (r *Registry) Validate(ctx context.Context, client *kubernetes.Clientset, logger *zap.SugaredLogger) error {
	if r.Local {
		return nil
	}
	validate := validator.New(validator.WithRequiredStructEnabled())
	for _, auth := range r.Config.Auths {
		err := validate.Struct(auth)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
	}
	if r.Exists(ctx, client) {
		return fmt.Errorf("secret for this registry server already exists")
	}
	return nil
}

// Check if registry in provided context exists
func (r *Registry) Exists(ctx context.Context, client *kubernetes.Clientset) bool {
	registries, _ := Registries(ctx, client)
	for _, registry := range registries {
		for authServer := range r.Config.Auths {
			if existsUrl := registry.Annotations[RegistryServerLabel]; existsUrl != "" && strings.Contains(existsUrl, authServer) {
				return true
			}
		}
		if existsUrl := registry.Annotations[RegistryServerLabel]; existsUrl != "" && strings.Contains(existsUrl, r.Annotations[RegistryServerLabel]) {
			return true
		}
	}
	return false
}

// Creates registry in requested context and assign it to engine
func (r *Registry) Create(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {
	var err error
	if r.Context != "" {
		config, err = cfg.GetConfigWithContext(r.Context)
		if err != nil {
			return err
		}
	}

	client, err := kube.Client(config)
	if err != nil {
		return err
	}
	installer, err := engine.GetEngine(config)
	if err != nil {
		return err
	}

	release, _ := installer.GetRelease()

	if r.Local {
		logger.Debug("Create Local Registry")
		err := r.CreateLocal(ctx, client)
		if err != nil {
			return err
		}
		r.Name = r.Domain()
		localRegistry := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kndp.io/v1alpha1",
				"kind":       "LocalRegistry",
				"metadata": map[string]interface{}{
					"name": r.Name,
				},
				"spec": map[string]interface{}{
					"name":                     r.Name,
					"namespace":                namespace.Namespace,
					"nodePort":                 string(nodePort),
					"kubernetesProviderCfgRef": engine.ProviderConfigName,
				},
			},
		}

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			return err
		}

		gvr := schema.GroupVersionResource{
			Group:    "kndp.io",
			Version:  "v1alpha1",
			Resource: "localregistries",
		}

		_, err = dynamicClient.Resource(gvr).Create(ctx, localRegistry, metav1.CreateOptions{})
		if err != nil {
			return err
		}

	} else {
		logger.Debug("Create Registry")
		r.Name = r.Domain()
		serverUrls := []string{}
		for _, auth := range r.Config.Auths {
			serverUrls = append(
				serverUrls,
				strings.Replace(auth.Server, "https://", "", -1),
			)
		}

		images := []map[string]interface{}{}
		for _, url := range serverUrls {
			images = append(
				images,
				map[string]interface{}{
					"<(image)": strings.Replace(url, "https://", "", -1) + "/*",
				},
			)
		}

		registry := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kndp.io/v1alpha1",
				"kind":       "Registry",
				"metadata": map[string]interface{}{
					"name": r.Name,
				},
				"spec": map[string]interface{}{
					"name":      r.Name,
					"namespace": namespace.Namespace,
					"server":    r.Config.Auths["server"].Server,
					"username":  r.Config.Auths["server"].Username,
					"password":  r.Config.Auths["server"].Password,
					"email":     r.Config.Auths["server"].Email,
					"images":    images,
					"imagePullSecrets": []interface{}{
						map[string]interface{}{
							"name": r.Name,
						},
					},
					"kubernetesProviderCfgRef": engine.ProviderConfigName,
				},
			},
		}

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			return err
		}
		policy.DeleteRegistryPolicy(ctx, config, &policy.RegistryPolicy{Name: r.Name, Urls: serverUrls})
		gvr := schema.GroupVersionResource{
			Group:    "kndp.io",
			Version:  "v1alpha1",
			Resource: "registries",
		}

		_, err = dynamicClient.Resource(gvr).Create(ctx, registry, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		if release.Config == nil {
			release.Config = map[string]interface{}{
				"imagePullSecrets": []interface{}{},
			}
		}
		if release.Config["imagePullSecrets"] == nil {
			release.Config["imagePullSecrets"] = []interface{}{}
		}
		release.Config["imagePullSecrets"] = append(
			release.Config["imagePullSecrets"].([]interface{}),
			r.Name,
		)
	}

	if r.Default {
		if release.Config["args"] == nil {
			release.Config["args"] = []interface{}{}
		}
		args := []string{}
		for _, arg := range release.Config["args"].([]interface{}) {
			if !strings.Contains(arg.(string), "--registry") {
				args = append(args, arg.(string))
			}
		}

		release.Config["args"] = append(
			args,
			"--registry="+r.Domain(),
		)
	}

	logger.Debug("Upgrade Corssplane chart", "Values", release.Config)

	return installer.Upgrade(engine.Version, release.Config)
}

func (r *Registry) FromSecret(sec corev1.Secret) *Registry {
	secJson, _ := json.Marshal(sec)
	json.Unmarshal(secJson, r)
	return r
}

func (r *Registry) ToSecret() *corev1.Secret {
	sec := corev1.Secret{}
	rJson, _ := json.Marshal(r)
	json.Unmarshal(rJson, &sec)
	return &sec
}

// Delete registry
func (r *Registry) Delete(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {

	installer, err := engine.GetEngine(config)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	release, _ := installer.GetRelease()

	if release.Config == nil || release.Config["imagePullSecrets"] == nil {
		logger.Warn("Not found any registry in context.")
	} else {
		oldRegistries := release.Config["imagePullSecrets"].([]interface{})

		newRegistries := []interface{}{}
		for _, reg := range oldRegistries {
			if reg != r.Name {
				newRegistries = append(
					newRegistries,
					reg,
				)
			}
		}
		release.Config["imagePullSecrets"] = newRegistries
		if len(oldRegistries) == len(newRegistries) {
			logger.Warn("Configuration URL not found applied configurations.")
			return nil
		}

		if r.Default {
			if release.Config["args"] != nil {
				args := []string{}
				for _, arg := range release.Config["args"].([]interface{}) {
					if !strings.Contains(arg.(string), "--registry") {
						args = append(args, arg.(string))
					}
				}

				release.Config["args"] = args
			}
		}

		err = installer.Upgrade(engine.Version, release.Config)
		if err != nil {
			return err
		}
	}

	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	if r.Local {
		r.DeleteLocal(ctx, client, logger)
	}

	err = policy.DeleteRegistryPolicy(ctx, config, &policy.RegistryPolicy{Name: r.Name})
	if err != nil {
		return err
	}

	return secretClient(client).Delete(ctx, r.Name, metav1.DeleteOptions{})
}

// Copy registries from source to destination contexts
func CopyRegistries(ctx context.Context, logger *zap.SugaredLogger, sourceConfig *rest.Config, destinationConfig *rest.Config) error {

	destClient, err := kube.Client(destinationConfig)
	if err != nil {
		return err
	}

	sourceClient, err := kube.Client(sourceConfig)
	if err != nil {
		return err
	}

	registries, err := Registries(ctx, sourceClient)
	if err != nil {
		return err
	}

	if len(registries) > 0 {
		for _, registry := range registries {
			if !registry.Exists(ctx, destClient) {
				registry.SetResourceVersion("")
				_, err = destClient.CoreV1().Secrets(namespace.Namespace).Create(ctx, registry.ToSecret(), metav1.CreateOptions{})
				if err != nil {
					return err
				}
			} else {
				logger.Warn("Registry for " + registry.Annotations[RegistryServerLabel] + " already exist inside of destination context.")
			}

		}
		logger.Info("Registries copied successfully.")
	} else {
		logger.Warn("Registries not found")
	}
	return nil
}

// Make registry default
func (r *Registry) SetDefault(d bool) {
	r.Default = d
}

// Make local registry
func (r *Registry) SetLocal(l bool) {
	r.Local = l
}

// Kubernetes context where registry will be created
func (r *Registry) WithContext(c string) {
	r.Context = c
}

// Domain of primary registry
func (r *Registry) Domain() string {
	if r.Local {
		return DefaultLocalDomain
	}
	domain := DefaultRemoteDomain
	domain = strings.Split(r.Config.Auths["server"].Server, "/")[2]
	return domain
}

func secretClient(client *kubernetes.Clientset) kv1.SecretInterface {
	return client.CoreV1().Secrets(namespace.Namespace)
}
