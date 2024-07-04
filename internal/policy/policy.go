package policy

import (
	"context"

	"k8s.io/client-go/rest"
)

type RegistryPolicy struct {
	Name string
	Urls []string
}

// Add policy controller
func AddPolicyConroller(ctx context.Context, config *rest.Config, plcType string) error {
	switch plcType {
	case "kyverno":
		err := addKyvernoPolicyConroller(ctx, config)
		if err != nil {
			return err
		}
		return addKyvernoDefaultPolicies(ctx, config)
	}
	return nil
}

// Add registry related policies
func AddRegistryPolicy(ctx context.Context, config *rest.Config, registry *RegistryPolicy) error {
	return addKyvernoRegistryPolicies(ctx, config, registry)
}

// Delete registry related policies
func DeleteRegistryPolicy(ctx context.Context, config *rest.Config, registry *RegistryPolicy) error {
	return deleteKyvernoRegistryPolicies(ctx, config, registry)
}
