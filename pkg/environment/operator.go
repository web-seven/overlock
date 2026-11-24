package environment

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/bootstrap"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/namespace"
)

// engineScopeSelector places engine components on the engine-scoped node of
// the k3s-docker engine.
var engineScopeSelector = map[string]string{scopeLabel: scopeEngine}

// Setup bootstraps the Overlock operator in the environment cluster and
// submits the Environment resource; the operator installs Crossplane,
// packages and tools. With overwrite, the Environment specification is
// applied; otherwise an existing Environment is kept. When a wait timeout is
// set, Setup reports progress until the Environment is ready.
func (e *Environment) Setup(ctx context.Context, overwrite bool, logger *zap.SugaredLogger) error {
	restConfig, err := config.GetConfigWithContext(e.context)
	if err != nil {
		return err
	}
	c, err := newClient(restConfig)
	if err != nil {
		return err
	}

	spec := e.environmentSpec()
	opts := bootstrap.Options{Image: e.operatorImage}
	if e.engine == "k3s-docker" {
		opts.NodeSelector = engineScopeSelector
	}

	logger.Infof("Installing Overlock operator (%s)", e.operatorImage)
	if err := bootstrap.InstallOperator(ctx, c, opts, logger); err != nil {
		return fmt.Errorf("failed to install operator: %w", err)
	}

	applied, err := bootstrap.ApplyEnvironment(ctx, c, e.name, spec, overwrite)
	if err != nil {
		return err
	}
	if applied {
		logger.Infof("Environment resource '%s' submitted", e.name)
	} else {
		logger.Infof("Environment resource '%s' already exists, keeping its specification", e.name)
	}

	if e.wait <= 0 {
		logger.Infof("Not waiting for readiness; check it with: kubectl get environment %s", e.name)
		return nil
	}
	logger.Info("Waiting for the operator to install components")
	if err := bootstrap.WaitReady(ctx, c, e.name, e.wait, logger); err != nil {
		return err
	}
	if spec.AdminServiceAccount != nil && applied {
		name := spec.AdminServiceAccount.Name
		if name == "" {
			name = kube.DefaultAdminServiceAccountName
		}
		logger.Infof("Admin service account '%s/%s' has cluster-admin privileges", namespace.Namespace, name)
	}
	return nil
}

// environmentSpec returns the Environment specification, scheduling engine
// components on the engine-scoped node for the k3s-docker engine.
func (e *Environment) environmentSpec() v1alpha1.EnvironmentSpec {
	spec := *e.spec.DeepCopy()
	if e.engine == "k3s-docker" && spec.Scheduling == nil {
		spec.Scheduling = &v1alpha1.SchedulingSpec{NodeSelector: engineScopeSelector}
	}
	return spec
}

// newClient returns a client that knows the core, CRD and Overlock types.
func newClient(restConfig *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, apiextensionsv1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			return nil, err
		}
	}
	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return c, nil
}
