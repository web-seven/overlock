package namespace

import (
	"context"

	"github.com/web-seven/overlock/internal/kube"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const OVERLOCK_ENGINE_NAMESPACE = "OVERLOCK_ENGINE_NAMESPACE"

var Namespace = "overlock"

// Creates system namespace
func CreateNamespace(ctx context.Context, config *rest.Config) error {
	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().Namespaces().Get(ctx, Namespace, v1.GetOptions{})
	if err != nil {
		ns := corev1.Namespace{ObjectMeta: v1.ObjectMeta{
			Name: Namespace,
		}}

		ns.SetResourceVersion("")
		_, err := client.CoreV1().Namespaces().Create(ctx, &ns, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
