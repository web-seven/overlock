package policy

import (
	"context"

	"k8s.io/client-go/rest"
)

func AddPolicyConroller(ctx context.Context, config *rest.Config, plcType string) error {
	switch plcType {
	case "kyverno":
		return AddKyvernoPolicyConroller(ctx, config)
	default:
		return nil
	}
}
