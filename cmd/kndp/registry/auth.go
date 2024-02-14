package registry

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type authCmd struct {
	RegistryServer string `required:"" help:"is your Private Registry FQDN."`
	Username       string `required:"" help:"is your Username."`
	Password       string `required:"" help:"is your Password."`
	Email          string `required:"" help:"is your Email."`
}

func (c *authCmd) Run(ctx context.Context, client *kubernetes.Clientset) error {
	var secretSpec coreV1.Secret
	secretSpec.ObjectMeta.GenerateName = "server-auth-"
	secretSpec.Data = map[string][]byte{
		".dockerconfigjson": []byte(""),
	}
	secretSpec.Type = "kubernetes.io/dockerconfigjson"
	secretsClient := client.CoreV1().Secrets("default")
	_, err := secretsClient.Create(ctx, &secretSpec, v1.CreateOptions{})
	if err != nil {
		panic(err)
	}

	return nil
}
