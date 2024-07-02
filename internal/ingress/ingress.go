package ingress

import (
	"context"

	"k8s.io/client-go/rest"
)

func AddIngressConroller(ctx context.Context, config *rest.Config, ingType string) error {
	switch ingType {
	case "nginx":
		return AddNginxIngressConroller(ctx, config)
	default:
		return nil
	}
}
