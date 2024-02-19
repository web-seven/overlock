package registry

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"strings"

	"github.com/kndpio/kndp/internal/configuration"
	"github.com/pterm/pterm"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type DockerRegistryConfig struct {
	Auths map[string]DockerRegistryAuth `json:"auths"`
}

type DockerRegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}

type authCmd struct {
	RegistryServer string `required:"" help:"is your Private Registry FQDN."`
	Username       string `required:"" help:"is your Username."`
	Password       string `required:"" help:"is your Password."`
	Email          string `required:"" help:"is your Email."`
}

type ReleaseConfig struct {
	Secrets map[string]string `json:"imagePullSecrets"`
}

func (c *authCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config) error {

	secretsClient := client.CoreV1().Secrets("default")

	secrets, _ := secretsClient.List(ctx, v1.ListOptions{LabelSelector: "kndp-registry-auth-config=true"})

	for _, existsSecret := range secrets.Items {
		if existsUrl := existsSecret.Annotations["kndp-registry-server-url"]; existsUrl != "" && strings.Contains(existsUrl, c.RegistryServer) {
			pterm.Info.Println("Secret for this registry server already exists.")
			return nil
		}
	}

	regConf, _ := json.Marshal(DockerRegistryConfig{
		Auths: map[string]DockerRegistryAuth{
			c.RegistryServer: {
				Username: c.Username,
				Password: c.Password,
				Email:    c.Email,
				Auth:     b64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password)),
			},
		},
	})

	secretSpec := coreV1.Secret{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "registry-server-auth-",
			Labels: map[string]string{
				"kndp-registry-auth-config": "true",
			},
			Annotations: map[string]string{
				"kndp-registry-server-url": c.RegistryServer,
			},
		},
		Data: map[string][]byte{".dockerconfigjson": regConf},
		Type: "kubernetes.io/dockerconfigjson",
	}

	secret, err := secretsClient.Create(ctx, &secretSpec, v1.CreateOptions{})
	if err != nil {
		return err
	}

	installer := configuration.GetManager(config)
	parameters := map[string]any{
		"imagePullSecrets": map[string]string{
			secret.ObjectMeta.Name: secret.ObjectMeta.Name,
		},
	}

	yamlParams, _ := yaml.Marshal(parameters)
	yaml.Unmarshal([]byte(yamlParams), parameters)

	err = installer.Upgrade("", parameters)
	if err != nil {
		return err
	} else {
		pterm.Success.Println("KNDP Authentication Secret created successfully.")
	}

	return nil
}
