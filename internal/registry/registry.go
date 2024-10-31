package registry

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/namespace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	cfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	RegistryServerLabel = "overlock-registry-server-url"
	DefaultRemoteDomain = "xpkg.upbound.io"
	LocalServiceName    = "registry"
	DefaultLocalDomain  = LocalServiceName + ".%s.svc.cluster.local"
	AuthConfigLabel     = "overlock-registry-auth-config"
)

type RegistryAuth struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Server   string `json:"server" validate:"required,http_url"`
	Auth     string `json:"auth"`
}

type RegistryConfig struct {
	Auths map[string]RegistryAuth `json:"auths"`
}

type Registry struct {
	Config  RegistryConfig
	Default bool
	Local   bool
	Context string
	Server  string
	Name    string
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
func New(server string, username string, password string, email string) Registry {
	registry := Registry{
		Default: false,
		Server:  server,
		Name:    "registry." + strconv.FormatInt(time.Now().UnixNano(), 10),
		Config: RegistryConfig{
			Auths: map[string]RegistryAuth{
				server: {
					Password: password,
					Username: username,
					Email:    email,
					Server:   server,
					Auth:     b64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
				},
			},
		},
	}
	registry.Annotations = map[string]string{
		RegistryServerLabel: server,
	}
	return registry
}

// Creates new local Registry
func NewLocal() Registry {
	registry := Registry{
		Default: false,
		Local:   true,
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
		err := r.CreateLocal(ctx, client, logger)
		if err != nil {
			return err
		}
	} else {
		logger.Debug("Create Registry")
		if release != nil {
			secretSpec := r.SecretSpec()
			secret, err := secretClient(client).Create(ctx, &secretSpec, metav1.CreateOptions{})
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
				secret.ObjectMeta.Name,
			)
		} else {
			logger.Debug("Crossplane engine not found!")
		}

	}

	if release != nil && r.Default {
		logger.Debug("Set registry as default.")
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

	if installer != nil && release != nil {
		logger.Debug("Upgrade Corssplane chart", "Values", release.Config)
		return installer.Upgrade(engine.Version, release.Config)
	} else {
		logger.Warnf("Crossplane engine not found, in namespace %s", namespace.Namespace)
	}
	return nil
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
		return fmt.Sprintf(DefaultLocalDomain, namespace.Namespace)
	}
	url, err := url.Parse(r.Server)
	if err != nil {
		log.Fatal(err)
	}
	return url.Hostname()
}

// Creates specs of Secret base on Registry data
func (r *Registry) SecretSpec() corev1.Secret {
	regConf, _ := json.Marshal(r.Config)
	secretSpec := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
			Labels: engine.ManagedLabels(map[string]string{
				"overlock-registry-auth-config": "true",
			}),
			Annotations: r.Annotations,
		},
		Data: map[string][]byte{".dockerconfigjson": regConf},
		Type: "kubernetes.io/dockerconfigjson",
	}

	return secretSpec
}

func secretClient(client *kubernetes.Clientset) kv1.SecretInterface {
	return client.CoreV1().Secrets(namespace.Namespace)
}
