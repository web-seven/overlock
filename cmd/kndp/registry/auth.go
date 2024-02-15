package registry

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"

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
		},
		Data: map[string][]byte{".dockerconfigjson": regConf},
		Type: "kubernetes.io/dockerconfigjson",
	}

	secretsClient := client.CoreV1().Secrets("default")
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
