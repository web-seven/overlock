package registry

import (
	"context"
	"encoding/json"

	"github.com/kndpio/kndp/internal/configuration"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type DockerRegistryConfig struct {
	Auths map[string]DockerRegistryAuth `json:"auths"`
}

type DockerRegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
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

	installer := configuration.GetCrossplaneChart(config)

	configs := ReleaseConfig{}
	parameters := map[string]any{}

	release, _ := installer.GetCurrentRelease()
	jsons, _ := json.Marshal(release.Config)
	json.Unmarshal(jsons, &configs)
	configs.Secrets[secret.Name] = secret.Name
	parameters["imagePullSecrets"] = configs.Secrets

	err = installer.Upgrade("", parameters)
	if err != nil {
		return err
	}

	return nil
}
