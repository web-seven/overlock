package engine

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
	"gopkg.in/yaml.v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretReconciler struct {
	serverIP string
	client.Client
	context.CancelFunc
}

const (
	RepoUrl            = "https://charts.crossplane.io/stable"
	ChartName          = "crossplane"
	ReleaseName        = "kndp-crossplane"
	Namespace          = "kndp-system"
	kindClusterRole    = "ClusterRole"
	clusterAdminRole   = "cluster-admin"
	providerConfigName = "kndp-kubernetes-provider-config"
	aggregateToAdmin   = "rbac.crossplane.io/aggregate-to-admin"
	trueVal            = "true"
)

var (
	managedLabels = map[string]string{
		"app.kubernetes.io/managed-by": "kndp",
	}
	initParameters = map[string]any{
		"provider": map[string]any{
			"packages": []string{
				"xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.13.0",
			},
		},
	}
)

// Get engine Helm manager
func GetEngine(configClient *rest.Config) (install.Manager, error) {
	repoURL, err := url.Parse(RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing repository URL: %v", err)
	}
	setWait := helm.InstallerModifierFn(helm.Wait())
	setNamespace := helm.InstallerModifierFn(helm.WithNamespace(Namespace))
	setUpInstall := helm.InstallerModifierFn(helm.WithUpgradeInstall(true))
	setCreateNs := helm.InstallerModifierFn(helm.WithCreateNamespace(true))

	installer, err := helm.NewManager(
		configClient,
		ChartName,
		repoURL,
		ReleaseName,
		setWait,
		setNamespace,
		setUpInstall,
		setCreateNs,
	)

	if err != nil {
		return nil, fmt.Errorf("error creating Helm manager: %v", err)
	}

	return installer, nil
}

// Install engine Helm release
func InstallEngine(ctx context.Context, configClient *rest.Config) error {
	engine, err := GetEngine(configClient)
	if err != nil {
		return err
	}

	err = engine.Upgrade("", initParameters)
	if err != nil {
		return err
	}

	return SetupPrivilegedKubernetesProvider(ctx, configClient)
}

// Check if engine release exists
func IsHelmReleaseFound(configClient *rest.Config) bool {

	installer, err := GetEngine(configClient)
	if err != nil {
		return false
	}
	_, err = installer.GetRelease()
	return err == nil

}

// Lables for engine system resources, mixed with provided
func ManagedLabels(m map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range managedLabels {
		merged[k] = v
	}
	for key, value := range m {
		merged[key] = value
	}
	return merged
}

// Selector for engine system resources, mixed with provided
func ManagedSelector(m map[string]string) string {
	selectors := []string{}
	for k, v := range managedLabels {
		selectors = append(selectors, k+"="+v)
	}
	for key, value := range m {
		selectors = append(selectors, key+"="+value)
	}
	return strings.Join(selectors, ",")
}

// Setup Kubernetes provider which has crossplane admin aggregation role assigned
func SetupPrivilegedKubernetesProvider(ctx context.Context, configClient *rest.Config) error {

	pcn := providerConfigName

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: Namespace,
		},
	}

	saSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: pcn,
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						aggregateToAdmin: trueVal,
					},
				},
			},
		},
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: pcn,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     kindClusterRole,
			Name:     cr.Name,
		},
	}

	pc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubernetes.crossplane.io/v1alpha1",
			"kind":       "ProviderConfig",
			"metadata": map[string]interface{}{
				"name": providerConfigName,
			},
			"spec": map[string]interface{}{
				"credentials": map[string]interface{}{
					"secretRef": map[string]interface{}{
						"key":       "kubeconfig",
						"name":      providerConfigName,
						"namespace": Namespace,
					},
					"source": "Secret",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	client, _ := ctrl.New(configClient, ctrl.Options{Scheme: scheme})
	for _, res := range []ctrl.Object{sa, saSec, cr, crb, pc} {
		_, err := controllerutil.CreateOrUpdate(ctx, client, res, func() error {
			return nil
		})
		if err != nil {
			return err
		}
	}

	svc := &corev1.Service{}
	err := client.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, svc)
	if err != nil {
		return err
	}

	mgr, err := manager.New(configClient, manager.Options{})
	if err != nil {
		return err
	}
	mgrContext, cancel := context.WithCancel(context.Background())
	if err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectNew.GetName() == providerConfigName
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetName() == providerConfigName
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Object.GetName() == providerConfigName
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Object.GetName() == providerConfigName
			},
		},
		).
		Complete(&SecretReconciler{
			Client:     client,
			CancelFunc: cancel,
			serverIP:   "https://" + svc.Spec.ClusterIP + ":443",
		}); err != nil {
		return err
	}
	mgr.Start(mgrContext)
	return nil
}

// Reconcile SvcAcc secret for make kubeconfig
func (a *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	sec := &corev1.Secret{}
	err := a.Get(ctx, req.NamespacedName, sec)
	if err != nil {
		return reconcile.Result{}, err
	} else if sec.GetName() != providerConfigName {
		return reconcile.Result{Requeue: true}, nil
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, a.Client, sec, func() error {
		kubeconfig, _ := yaml.Marshal(&map[string]interface{}{
			"apiVersion":      "v1",
			"kind":            "Config",
			"current-context": "in-cluster",
			"clusters": []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"certificate-authority-data": b64.StdEncoding.EncodeToString(sec.Data["ca.crt"]),
						// "certificate-authority-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMvakNDQWVhZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRJME1EUXdPVEV6TXpZek9Gb1hEVE0wTURRd056RXpNell6T0Zvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTFRQCi9peE5WbVlUNEEyaXZZeVk4dHdWbUxLWkxwZkFFSW1aY3VDcTJQZVg0R3NCS3kyc012SXA3eHhudkFjTUN1RGUKRG9sdGpOenBZSm9nSTduaG5OOVpWYlJvMytPVlJNamtDR0dvL1dYcTBsdVpCekV3WTJNdkx2ZDA5TUJ0aXJvQgo3d2NycnpGUmxBR1BGQkIxQmxwSHhUT3p3bVRGZHlGcnZMeXh0TllGNTNvV1l3WTZLWnVzTnc0NUtVZmV6OTN6CnRGSStYS3EzLzJBajVISTJHV09WYjM4aHFQZ0xEbEdWOUtzdm1CMkpVSi9tSUY3bUlJZ2hKU1BjTWJ2QllobW8KdFU0WnBjTGdLWkozajFCR1lsYWFrS3EwWGdKVk9PK2lqV0UyZ1hDK0d6Tm5keU93MlBPTjFKYVBLMTEyeFRFVwpEQ0FsQi9Uc2dyR3FZcUVQMU5FQ0F3RUFBYU5aTUZjd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0hRWURWUjBPQkJZRUZKdVg4MDhVV2JZQnJTbDV5Qi9QaElNeU5wSTBNQlVHQTFVZEVRUU8KTUF5Q0NtdDFZbVZ5Ym1WMFpYTXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBS2dhRkkwZnlRVUF3MnVwZnV3WQpiSzNqOG9sRXZ5UGptZ2R1bXVGRnlaeDhFVkExbkFQNkdXOHV2RFpzeHFjSEgrNmZNNkVOVVlHWEpsZS9lRlZHCjVMbFJSeHdURDN2RUVob2V6RDNqMHM0V1IvVC9DUk9aYWQ5MUUvSW1HemdIZU9DL0JqV2VqNkJYVkFPVnRiNnAKN3kvam44cHN5WVJRL0p5eExwcTJvRzM5NmN0dnVxYXdUMy9mYzB1ZEhCMG5XTXpaR3ZhVHpHQ1JkbGpzSXEreQp3UlZGYkUrenJHTjVxV1dMOTFzNWhPZXdxVHdObW0wVHFuSGhSQUoxNXZhU2MvbS9SellyM2NlcXVPcjVMZmJhCmFRZmVpZ01nc2c4b05EU0w2NitqbW9UUkI2ZHpSR3ozVXhZTmZDZmdvbEZZV3dNS1RETSsrQWRESHJpck4vOGsKcW13PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==",
						// "certificate-authority-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMvakNDQWVhZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRJME1EUXhNekUwTlRNMU5sb1hEVE0wTURReE1URTBOVE0xTmxvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTE51CnNRc3VSZEZWNTJsZmJsNFFrRCtaSFpNSG84MTBDRlU0RUYyd1c4bUpLY3EwbW8zZTVHcUJISUlObHAzSFlFSTQKd1BNWGZLeFNYcmE3UDk4NGNDVElVbDc0Mi9tY3pNZVVDZkRxMFpGUXA4am5iZVJ1bHdDMmx2ampsU2NaZXJYaQo2end2dXhpZlRuQVZkSHJoSFduVktZQjZQNm5ndjRsMld4ekFvSWVQNDRhK1doc3hGT0x3QkR5ZWNXZTYzeit1CjdocFdyK01ZYWZZSVAvTzVsV2Z5U3Y2M3hwUHVBUEpLZGU5cVNXekxvZTRQMUw0MGI5ajVNa3pSM3RXdFIweUoKa3k2cGZIbnBmcys2ZCtYQkxldnBxMW94cjhVZkxPamFsdzdyNG9vVmhpOHNEUkxNdlBOUDZWZFF3OEUyZGZOSgpvMzVjZ3JDaU04aUVzWFF1YU1jQ0F3RUFBYU5aTUZjd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0hRWURWUjBPQkJZRUZFNkMrYklvb05TSFZVRWk3SmcxWjVrWEhDWWJNQlVHQTFVZEVRUU8KTUF5Q0NtdDFZbVZ5Ym1WMFpYTXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSFdFaTdMdFlodWpqYmhMckdjMApMcjlpUDVVK1cvWmFLcXNHbDd5NXhxQmdCckxNMVllamM4Ny9qNmQ5R2QreHM4aFdlcHhnNkErVjBBbmdXSStFCjNsMWhpS0xiQm5kWitRU1d2S002WEl0allvSmx0TjJoeXEzRk1TNCtyb3c4RXd2TFNvNUZKZlJPdWlKQ1JyMWQKRFVhV2NjRFlKMHJYNXJoQVphcDk0QjUwZHRuUURWL2xXRWVQZlJiY0Q0Mk5RRThtQ0ZkMDBxVWp2OGwyaTVnYQpNemZWcFU4b09LTXExMExxRGtxeDlnaFN6bC9OR0Y5SEVwcTlObXBJSjZnZUtZVklmMGtYdGZsNDIzdmo3cHRzCnR2YnJwVllXbWY3eVRYQTE1NTl6OGFYNmJtdndiMjE2TU95bTY1QVl2YlBmU0hNbUhOemJYaGo5NVU1bVNPdEgKUFZ3PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==",
						"server": a.serverIP,
					},
					"name": "in-cluster",
				},
			},
			"contexts": []map[string]interface{}{
				{
					"context": map[string]interface{}{
						"cluster":   "in-cluster",
						"user":      "in-cluster",
						"namespace": "kndp-system",
					},
					"name": "in-cluster",
				},
			},
			"preferences": map[string]interface{}{},
			"users": []map[string]interface{}{
				{
					"name": "in-cluster",
					"user": map[string]interface{}{
						"token": string(sec.Data["token"]),
						// "client-certificate-data": b64.StdEncoding.EncodeToString(sec.Data["ca.crt"]),
						// "token": "eyJhbGciOiJSUzI1NiIsImtpZCI6Inp2ZDVOdjh6YjZWQUVfdk1fVnhjU284cHY2bmctcmNhQkZ3S0MwQU1neEEifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrbmRwLXN5c3RlbSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJrbmRwLWt1YmVybmV0ZXMtcHJvdmlkZXItY29uZmlnIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6ImtuZHAta3ViZXJuZXRlcy1wcm92aWRlci1jb25maWciLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiI1YWQ2N2MyYi00MjUxLTQ4NzUtYWI4MC0wYmNmNTVjZjlhYjMiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6a25kcC1zeXN0ZW06a25kcC1rdWJlcm5ldGVzLXByb3ZpZGVyLWNvbmZpZyJ9.Xe9bjsyt28zLQt27jnHGp3htkyDDQ1-lKTVkSwnc6Okg3AY04SK4GNvTddv_H6M_P3azYw-O07bQpcbCGAwGyG5wcwcEFi2VzrOu9H1NauNNH4qpMB0HGrNuw_UE22iOySqWNhGTgAqMbuhN7q1pLOiO6vwdQvH2zltUC0v_AQWJQ8nse0WmX236kKLyFOf34YqaC_lDcbyqLu4k_Aessbz5w3sRRRjCyD-IYyWDx9Ahb0GtJndp8EwI7rurE546RE0PhcaJl45ucciAxyEn1Odlk4voMWZPcoycEHA5DCdV3iqrce5vdCB530NoRcHU0XKvDay-KlneqynPf_WM9Q",

						// "client-certificate-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURJVENDQWdtZ0F3SUJBZ0lJTm55MmVwQ3JpYWt3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBME1Ea3hNek0yTXpoYUZ3MHlOVEEwTURreE16TTJOREJhTURReApGekFWQmdOVkJBb1REbk41YzNSbGJUcHRZWE4wWlhKek1Sa3dGd1lEVlFRREV4QnJkV0psY201bGRHVnpMV0ZrCmJXbHVNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQW93Umo0eENiQ25PbWlzWWgKM0pTTTVBbHJuZkdDSHJGSjlpREViWmxpUG5FQXQyZjdSRmlKbi9sM2VMSHZpYTkxZFB4NTRkQTBSRXRYZzFibgplSHE2cDRhbXFraHF2RWUrdE5vdllwT3dpMFBmYndMMHdlL0NEOCt2REFHblRRaUpYeWQ1VFhQUEwvRVdNWm9yCk9lSUZqRmJUY3NxbVEvR1ozTGR2YlhUcnpSbFhoOTIxT1lESktFNUg2bm9JWHVIdjFMYVFuMnlnbUhBMTJUYUIKaHh3TU5aNmZNOWxjVlRUUWc2ODB1N1hIYmc2WHBUSElwSnRiM0lnR1RBWmF2QmVzVXl4UFJiSHZtZU05a1d0NApiRGdVNS8vWFJYTnNValRLdXRGSEErc2FKcm1mZ2kzOVM3c2dweXlVTHNKRkMvN05Yb2ZjU01nVnBEZTNJbDd4CmI2eWNld0lEQVFBQm8xWXdWREFPQmdOVkhROEJBZjhFQkFNQ0JhQXdFd1lEVlIwbEJBd3dDZ1lJS3dZQkJRVUgKQXdJd0RBWURWUjBUQVFIL0JBSXdBREFmQmdOVkhTTUVHREFXZ0JTYmwvTlBGRm0yQWEwcGVjZ2Z6NFNETWphUwpOREFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBcG4xOEZXMmZUVmpzc0JVeG52YzJLL0JDTjlSZTJIUzJncU10CmZnN2lPcFc3QnExaXZRc0NyTHRYRlFxTWVuZU5KZ3lFQkhYUGRkUFZqbmZzSFRFMXlldzNIUFJRTS9ISDkvaW4KZzcrNzMvczJYUDVBY09iRFFpUFNCcGllWWRjNFhDYjduZWtENXZwUFZEUDdTTHFSeWNwWHNnbWpTV25MeHBERQpNMUFpcE53YzhqWVFueHM0N1AyUHUxb1BaRTJ1QmJ3enUxdzhKR0xKZnJ4dTdoekw3NUFBUk94U0FDOGZPTW5nCnJCYjhSVUFUaGJqZHpiTGtZQlU5bG9SS011bmdlNjh2QXRIMmJaUTJzT3hndHluWFFZSkExUlRsbTIrUnpPbTAKblpnOTFXeEVBUTl3dGxMRVFVZ2RZWUJha3ltbTMrc0Uzbzgvc3htN2t2MzhFZVR3RWc9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==",
						// "client-key-data":         "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb2dJQkFBS0NBUUVBb3dSajR4Q2JDbk9taXNZaDNKU001QWxybmZHQ0hyRko5aURFYlpsaVBuRUF0MmY3ClJGaUpuL2wzZUxIdmlhOTFkUHg1NGRBMFJFdFhnMWJuZUhxNnA0YW1xa2hxdkVlK3ROb3ZZcE93aTBQZmJ3TDAKd2UvQ0Q4K3ZEQUduVFFpSlh5ZDVUWFBQTC9FV01ab3JPZUlGakZiVGNzcW1RL0daM0xkdmJYVHJ6UmxYaDkyMQpPWURKS0U1SDZub0lYdUh2MUxhUW4yeWdtSEExMlRhQmh4d01OWjZmTTlsY1ZUVFFnNjgwdTdYSGJnNlhwVEhJCnBKdGIzSWdHVEFaYXZCZXNVeXhQUmJIdm1lTTlrV3Q0YkRnVTUvL1hSWE5zVWpUS3V0RkhBK3NhSnJtZmdpMzkKUzdzZ3B5eVVMc0pGQy83TlhvZmNTTWdWcERlM0lsN3hiNnljZXdJREFRQUJBb0lCQUMyUE1JdHBQS3R6SHZ4eAoyMHpXaDNuRDJEdlFIMW1NbXVzYXhVc01MeFRjYUNMYUFMTmRPemxtY3lsY01XSDlrNG9hZGNYU2Rva1B0V21UCmhDVjd4MmJDanhuUUcyUjdlS1Q2eFh0N1l6L0l2RTArT2tGcFRJYzJ0K2xYSFBhK2lBWDc5ajdiT3ZCZkpLREEKUVl4dnlyVXFIdlphQkpYQWxBdkhpSERDMkpQOUt2RUpWNjlVSG1LUUIyajErQ0oxc28wakZKeTdNSzZNNEduZQo1bUhGU3JtYzFnYjcramRCSHBFTkdoeHZJL2YzVDMxNGlzMzQ3YlRmeUszdnQwNEZoWkhyeUhSUG1yVGo5NlNoClFKb0EvUGRIZWVwMTdGeERHVjFadkhmbFpjbHAwditrZDI5RXFjR2x6WGp6RWdzak9jQXlMWEtjaER5MnZVazQKRUgrK2ZlRUNnWUVBejk5SndiSHdnNjkyZ3g3aVZLR252T1FTRUlUSm1zRFh6U1VkWFRJRGpSNzJ6SGIrZGdJcgpoVFlsZThZQlowL0trTUk3dkZkR2dwQmtHYW10OWoxUU1IZG1yTG9GbjhtMGRWZjlJVGMzWFNzMXVTdFJoWEt4ClVtOXVzV3ZYVHgySmM2aitrSTVWR0VTWEc3RHQvMUFMQzFVYkNQRlF2Mm5QQmd0RGdmVmI5cGNDZ1lFQXlNS0QKMXdIa1JyaHVoZjlYODgyVkpXMDh4SW1QVXR1MThVMkVpVmtEaGpROG1RczFrSks1WUlSeGxRdTI4UXJ1alZVOApRNDh4c0FNM1c4RUliWGpYYkZMeW0zR3M0bk9RRGJWNkhySHZBbTBQTTlLdlhUTFN1VTc1eWRnN29hNVlGVnBsCkRZaUNRRzJNUWh4dVQ3WHp6U3VrTlBIcExpNW5OZENsSElPTnliMENnWUIxYjk5NmQyMjY1OGtiZ0xvN04rek0KMFVqSFhrMkxpVUVoMjhNQUlMNVMzdGh0WVJpWFVOaUhkTFN1ZlluVGRRZXF5cUQyNFpPck5hbm51YTNYUElKdQpMemFwaEpxaTBGQ01McjZLSW1pNzBTcVR4ejVTRng3SXhMMlRyS3BDUHh5bFpDY1ZRZVFmUnJqYjR4UkNObFZXCi9LaStYNTdQMVJZcGd2bUxsVE4wVndLQmdHVVdCRnB6bW1TOW92RVhwRXFmZm5UTTd5Y3ErSjdKQUhEVERtUTIKRE44N1dEUGJnQW9leHZiQldZdXB6V0RMbDFoVXpiWmEyTEwrdTVZWXVVeWQ1eUtsRllHTm1IYWhwNnd2YjZFYgpDUFRZd3luZDhPemxsVk0zWC9EeFR2MVhFd1VWY2dLQmRNeEtITENCTGs0Mm1ONzdGWUNQT2xGRmpqUjdyVmVSCktnaWxBb0dBWkJSeTQ5SUgxUGpZQmpTdDhZSThEODhFZ2NzK2lkQlhXaU81RG95L1VreENNdnNSak5OTWtaUGEKM3l1QVkwVEVlOER2b0ZEUWxFMGhaZnpYQ29WOFNFcGRSdnRNMzlLWm1Qd2FIaWFqOEVSUm03WTlSckI2eFNJMApDdWVVd0xaR3FYNzBvNjdLUDBxbnNkQVpZWmFIdm9zZnkyZDRiUWVkcWFGQUlkVkYxU0k9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==",
					},
				},
			},
		})

		sec.Data["kubeconfig"] = []byte(kubeconfig)
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// apiVersion: v1
	// kind: Config
	// current-context: "kind-source"
	// clusters:
	// - cluster:
	//     certificate-authority-data: ""
	// 		server: https://35.234.64.110
	//   	name: gke_break-packages_europe-west3-a_break-packages
	// contexts:
	// - context:
	//     cluster: kind-source
	//     user: kind-source
	//   name: kind-source
	// users:
	// - name: kind-source
	//   user:
	//     client-certificate-data: ""

	a.CancelFunc()

	return reconcile.Result{}, nil
}
