package provider

import (
	"context"

	"go.uber.org/zap"

	provider "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/web-seven/overlock/internal/image"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/packages"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Provider struct {
	Name    string
	Image   image.Image
	Upgrade bool
	Apply   bool
	packages.Package
}

// New Provider entity
func New(name string) *Provider {
	return &Provider{
		Name:  name,
		Image: image.Image{Image: empty.Image},
	}
}

func (p *Provider) WithUpgrade(upgrade bool) *Provider {
	p.Upgrade = upgrade
	return p
}

func (p *Provider) WithApply(apply bool) *Provider {
	p.Apply = apply
	return p
}

// Get list of providers from k8s context
func ListProviders(ctx context.Context, dynamicClient dynamic.Interface, logger *zap.SugaredLogger) []provider.Provider {

	destConf, _ := kube.GetKubeResources(kube.ResourceParams{
		Dynamic:    dynamicClient,
		Ctx:        ctx,
		Group:      "pkg.crossplane.io",
		Version:    "v1",
		Resource:   "providers",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	})
	var providers []provider.Provider
	for _, conf := range destConf {
		var paramsProvider provider.Provider
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(conf.UnstructuredContent(), &paramsProvider); err != nil {
			logger.Errorf("Failed to convert item %s: %v\n", conf.GetName(), err)
			continue
		}
		providers = append(providers, paramsProvider)
	}
	return providers
}

// Upgrade provider patch version
// according to providers from k8s context
// with same package and same minor version
func (p *Provider) UpgradeProvider(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	prvs := ListProviders(ctx, dc, logger)
	var pkgs []packages.Package
	for _, c := range prvs {
		pkg := packages.Package{
			Name: c.Name,
			Url:  c.Spec.Package,
		}
		pkgs = append(pkgs, pkg)
	}
	var err error
	p.Name, err = p.UpgradeVersion(ctx, dc, p.Name, pkgs)
	if err != nil {
		return err
	}
	return nil
}
