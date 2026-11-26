package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/bootstrap"
	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/namespace"
)

const (
	// LocalCABundleName is the ConfigMap with the CA of the local registry
	// that Crossplane trusts.
	LocalCABundleName = "overlock-registry-ca"
	localCABundleKey  = "ca.crt"
	localTLSSecret    = "registry-tls"
	localTLSTimeout   = 2 * time.Minute
	rewriteRuleInfix  = "-rewrite-"
	maxRuleNameLength = 50
)

// ruleName returns the Environment rule name prefix of the registry.
func (r *Registry) ruleName() string {
	name := engine.ToDNSLabel(strings.ToLower(r.Name))
	if len(name) > 30 {
		name = strings.Trim(name[:30], "-")
	}
	return name
}

// ownsRule reports whether an Environment rule belongs to the registry.
func (r *Registry) ownsRule(rule v1alpha1.RegistrySpec) bool {
	return rule.Name == r.ruleName() || strings.HasPrefix(rule.Name, r.ruleName()+rewriteRuleInfix)
}

// environmentRules returns the Environment registry rules of the registry:
// authentication with its pull secret for a remote registry, and a rewrite
// rule for each prefix redirected to the registry.
func (r *Registry) environmentRules() ([]v1alpha1.RegistrySpec, error) {
	domain, err := r.Domain()
	if err != nil {
		return nil, err
	}
	pullSecret := ""
	var rules []v1alpha1.RegistrySpec
	if !r.Local {
		pullSecret = r.Name
		rules = append(rules, v1alpha1.RegistrySpec{
			Name:       r.ruleName(),
			Prefixes:   []string{domain + "/"},
			PullSecret: pullSecret,
		})
	}
	for _, prefix := range r.Rewrites {
		rewrite, err := rewritePrefix(prefix, domain)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(prefix))
		rules = append(rules, v1alpha1.RegistrySpec{
			Name:          r.ruleName() + rewriteRuleInfix + hex.EncodeToString(sum[:])[:8],
			Prefixes:      []string{prefix},
			RewritePrefix: rewrite,
			PullSecret:    pullSecret,
		})
	}
	return rules, nil
}

// rewritePrefix returns the prefix that replaces a fully qualified image
// prefix to pull the same repositories from domain: the registry host of the
// prefix is replaced, the repository path kept. For example
// "xpkg.crossplane.io/crossplane-contrib/" becomes
// "registry.overlock.svc.cluster.local/crossplane-contrib/".
func rewritePrefix(prefix, domain string) (string, error) {
	host, path, _ := strings.Cut(prefix, "/")
	if host == "" || !strings.ContainsAny(host, ".:") {
		return "", fmt.Errorf("rewrite prefix %q must start with a registry host, e.g. xpkg.crossplane.io/crossplane-contrib/", prefix)
	}
	return domain + "/" + path, nil
}

// newClient returns a client for registry and Environment resources.
func newClient(config *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, apiextensionsv1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			return nil, err
		}
	}
	return client.New(config, client.Options{Scheme: scheme})
}

// ApplyRules replaces the Environment rules of the registry with its current
// rewrites (see applyToEnvironment).
func (r *Registry) ApplyRules(ctx context.Context, config *rest.Config) error {
	return r.applyToEnvironment(ctx, config)
}

// applyToEnvironment replaces the Environment rules of the registry and, for
// the local registry, makes Crossplane trust its certificate.
func (r *Registry) applyToEnvironment(ctx context.Context, config *rest.Config) error {
	rules, err := r.environmentRules()
	if err != nil {
		return err
	}
	c, err := newClient(config)
	if err != nil {
		return err
	}
	var caBundle *v1alpha1.ConfigMapKey
	if r.Local {
		if caBundle, err = ensureLocalCABundle(ctx, c); err != nil {
			return err
		}
	}
	return bootstrap.UpdateEnvironment(ctx, c, func(spec *v1alpha1.EnvironmentSpec) {
		spec.Registries = append(r.otherRules(spec.Registries), rules...)
		if caBundle != nil {
			spec.Crossplane.RegistryCABundle = caBundle
		}
	})
}

// removeFromEnvironment removes the Environment rules of the registry and, for
// the local registry, its certificate trust.
func (r *Registry) removeFromEnvironment(ctx context.Context, config *rest.Config) error {
	c, err := newClient(config)
	if err != nil {
		return err
	}
	err = bootstrap.UpdateEnvironment(ctx, c, func(spec *v1alpha1.EnvironmentSpec) {
		spec.Registries = r.otherRules(spec.Registries)
		if r.Local && spec.Crossplane.RegistryCABundle != nil && spec.Crossplane.RegistryCABundle.Name == LocalCABundleName {
			spec.Crossplane.RegistryCABundle = nil
		}
	})
	if err != nil {
		return err
	}
	if r.Local {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: LocalCABundleName, Namespace: namespace.Namespace}}
		return client.IgnoreNotFound(c.Delete(ctx, cm))
	}
	return nil
}

func (r *Registry) otherRules(rules []v1alpha1.RegistrySpec) []v1alpha1.RegistrySpec {
	var out []v1alpha1.RegistrySpec
	for _, rule := range rules {
		if !r.ownsRule(rule) {
			out = append(out, rule)
		}
	}
	return out
}

// ensureLocalCABundle copies the CA of the local registry certificate, issued
// by cert-manager, into a ConfigMap Crossplane can use as CA bundle.
func ensureLocalCABundle(ctx context.Context, c client.Client) (*v1alpha1.ConfigMapKey, error) {
	var ca []byte
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, localTLSTimeout, true, func(ctx context.Context) (bool, error) {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: localTLSSecret}, secret); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		ca = secret.Data[localCABundleKey]
		return len(ca) > 0, nil
	})
	if err != nil {
		return nil, fmt.Errorf("local registry certificate %s/%s is not ready: %w", namespace.Namespace, localTLSSecret, err)
	}

	cm := &corev1.ConfigMap{}
	err = c.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: LocalCABundleName}, cm)
	switch {
	case apierrors.IsNotFound(err):
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: LocalCABundleName, Namespace: namespace.Namespace, Labels: engine.ManagedLabels(nil)},
			Data:       map[string]string{localCABundleKey: string(ca)},
		}
		err = c.Create(ctx, cm)
	case err == nil:
		cm.Data = map[string]string{localCABundleKey: string(ca)}
		err = c.Update(ctx, cm)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to save local registry CA bundle: %w", err)
	}
	return &v1alpha1.ConfigMapKey{Name: LocalCABundleName, Key: localCABundleKey}, nil
}
