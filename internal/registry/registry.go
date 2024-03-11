package registry

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-playground/validator/v10"
	"github.com/kndpio/kndp/internal/configuration"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type RegistryAuth struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Auth     string `json:"auth"`
}

type RegistryConfig struct {
	Auths map[string]RegistryAuth `json:"auths"`
}

type Registry struct {
	Name   string
	Config RegistryConfig
}

func Registries(ctx context.Context, client *kubernetes.Clientset) (*corev1.SecretList, error) {
	return secretClient(client).
		List(ctx, v1.ListOptions{LabelSelector: "kndp-registry-auth-config=true"})
}

func (r *Registry) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	for serverUrl, auth := range r.Config.Auths {

		err := validate.Var(serverUrl, "required,http_url")
		if err != nil {
			return err
		}

		err = validate.Struct(auth)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) Exists(ctx context.Context, client *kubernetes.Clientset) bool {
	secrets, _ := Registries(ctx, client)
	for _, existsSecret := range secrets.Items {
		for authServer, _ := range r.Config.Auths {
			if existsUrl := existsSecret.Annotations["kndp-registry-server-url"]; existsUrl != "" && strings.Contains(existsUrl, authServer) {
				return true
			}
		}
	}
	return false
}

func (r *Registry) Secret() corev1.Secret {
	regConf, _ := json.Marshal(r.Config)
	servers := []string{}
	for authServer, _ := range r.Config.Auths {
		servers = append(servers, authServer)
	}

	secretSpec := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "registry-server-auth-",
			Labels: map[string]string{
				"kndp-registry-auth-config": "true",
			},
			Annotations: map[string]string{
				"kndp-registry-server-url": strings.Join(servers, ","),
			},
		},
		Data: map[string][]byte{".dockerconfigjson": regConf},
		Type: "kubernetes.io/dockerconfigjson",
	}

	return secretSpec
}

func (r *Registry) Create(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *log.Logger) error {
	secretSpec := r.Secret()
	secret, err := secretClient(client).Create(ctx, &secretSpec, v1.CreateOptions{})
	if err != nil {
		return err
	}

	installer := configuration.GetManager(config, logger)
	release, _ := installer.GetRelease()

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

	logger.Debug("Upgrade Corssplane chart", "Values", release.Config)

	return installer.Upgrade("", release.Config)
}

func (r *Registry) Delete(ctx context.Context, client *kubernetes.Clientset) error {
	return secretClient(client).Delete(ctx, r.Name, v1.DeleteOptions{})
}

func secretClient(client *kubernetes.Clientset) kv1.SecretInterface {
	return client.CoreV1().Secrets("default")
}
