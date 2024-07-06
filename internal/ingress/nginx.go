package ingress

import (
	"context"
	"net/url"

	"github.com/kndpio/kndp/internal/install/helm"
	"k8s.io/client-go/rest"
)

const (
	nginxChartName    = "ingress-nginx"
	nginxChartVersion = "4.10.1"
	nginxReleaseName  = "ingress-nginx"
	nginxRepoUrl      = "https://kubernetes.github.io/ingress-nginx"
	nginxNamespace    = "ingress-nginx"
)

var (
	chartValues = map[string]interface{}{
		"controller": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				"ingress-ready": "true",
			},
			"terminationGracePeriodSeconds": "0",
			"watchIngressWithoutClass":      true,
			"publishService": map[string]interface{}{
				"enabled": false,
			},
			"tolerations": []map[string]interface{}{
				{
					"key":      "node-role.kubernetes.io/master",
					"operator": "Equal",
					"effect":   "NoSchedule",
				},
				{
					"key":      "node-role.kubernetes.io/control-plane",
					"operator": "Equal",
					"effect":   "NoSchedule",
				},
			},
			"hostPort": map[string]interface{}{
				"enabled": true,
			},
			"service": map[string]interface{}{
				"type": "NodePort",
			},
			"extraArgs": map[string]interface{}{
				"publish-status-address": "localhost",
			},
		},
	}
)

func AddNginxIngressConroller(ctx context.Context, config *rest.Config) error {
	repoURL, err := url.Parse(nginxRepoUrl)
	if err != nil {
		return err
	}

	manager, err := helm.NewManager(config, nginxChartName, repoURL, nginxReleaseName,
		helm.InstallerModifierFn(helm.WithNamespace(nginxNamespace)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(true)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(true)),
	)
	if err != nil {
		return err
	}

	manager.Upgrade(nginxChartVersion, chartValues)
	if err != nil {
		return err
	}

	return nil
}
