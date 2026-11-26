package controller

import (
	"errors"
	"fmt"
	"net/url"

	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/install"
	"github.com/web-seven/overlock/internal/install/helm"
)

// Releases manages the Helm releases of components.
type Releases interface {
	// Get returns the release of c, or nil when it does not exist.
	Get(c helmComponent) (*release.Release, error)
	// Install installs c with the given release labels.
	Install(c helmComponent, labels map[string]string) error
	// Upgrade upgrades c to its version and values with the given release labels.
	Upgrade(c helmComponent, labels map[string]string) error
	// Uninstall removes the release of c.
	Uninstall(c helmComponent) error
}

// helmReleases implements Releases with the Helm SDK.
type helmReleases struct {
	restConfig *rest.Config
}

// NewHelmReleases returns Releases backed by Helm for the cluster of restConfig.
func NewHelmReleases(restConfig *rest.Config) Releases {
	return &helmReleases{restConfig: restConfig}
}

func (h *helmReleases) manager(c helmComponent, labels map[string]string, extra ...helm.InstallerModifierFn) (install.Manager, error) {
	repoURL, err := url.Parse(c.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL %q: %w", c.RepoURL, err)
	}
	modifiers := append([]helm.InstallerModifierFn{
		helm.Wait(),
		helm.WithNamespace(c.Namespace),
		helm.WithCreateNamespace(true),
		helm.WithUpgradeInstall(false),
		helm.WithReuseValues(false),
		helm.WithLabels(labels),
	}, extra...)
	return helm.NewManager(h.restConfig, c.Chart, repoURL, c.Release, modifiers...)
}

func (h *helmReleases) Get(c helmComponent) (*release.Release, error) {
	mgr, err := h.manager(c, nil)
	if err != nil {
		return nil, err
	}
	rel, err := mgr.GetRelease()
	if errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, nil
	}
	return rel, err
}

func (h *helmReleases) Install(c helmComponent, labels map[string]string) error {
	mgr, err := h.manager(c, labels)
	if err != nil {
		return err
	}
	return mgr.Install(c.Version, c.Values)
}

func (h *helmReleases) Upgrade(c helmComponent, labels map[string]string) error {
	mgr, err := h.manager(c, labels)
	if err != nil {
		return err
	}
	return mgr.Upgrade(c.Version, c.Values)
}

func (h *helmReleases) Uninstall(c helmComponent) error {
	var extra []helm.InstallerModifierFn
	if c.UninstallWithoutHooks {
		extra = append(extra, helm.WithNoHooks())
	}
	mgr, err := h.manager(c, nil, extra...)
	if err != nil {
		return err
	}
	return mgr.Uninstall()
}
