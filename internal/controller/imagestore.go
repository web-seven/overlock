package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/namespace"
)

const (
	componentImageStore = "image-store"
	imageStoreName      = "overlock-image-store"
	// imageStoreService is the Service name, so the store is reached as the
	// local registry domain (registry.<namespace>.svc.cluster.local).
	imageStoreService = "registry"
	imageStoreSecret  = imageStoreName + "-tls"
	// ImageStoreCA is the ConfigMap with the CA of the image store, trusted by
	// Crossplane.
	ImageStoreCA      = imageStoreName + "-ca"
	imageStoreCAKey   = "ca.crt"
	imageStorePort    = 8443
	defaultSocketPath = "/run/k3s/containerd/containerd.sock"
)

var (
	certificateGVK = schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"}
	issuerGVK      = schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Issuer"}
)

func imageStoreLabels(env string) map[string]string {
	labels := ownerLabels(env)
	labels[labelComponent] = componentImageStore
	return labels
}

// imageStoreCA issues the TLS certificate of the image store with cert-manager
// and copies its CA into a ConfigMap for Crossplane. It returns the CA
// ConfigMap key once the certificate is issued, nil before.
func (r *EnvironmentReconciler) imageStoreCA(ctx context.Context, env *v1alpha1.Environment) (*v1alpha1.ConfigMapKey, error) {
	labels := imageStoreLabels(env.Name)

	issuer := &unstructured.Unstructured{}
	issuer.SetGroupVersionKind(issuerGVK)
	issuer.SetName(imageStoreName)
	issuer.SetNamespace(namespace.Namespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, issuer, func() error {
		issuer.SetLabels(labels)
		return unstructured.SetNestedMap(issuer.Object, map[string]any{}, "spec", "selfSigned")
	}); err != nil {
		return nil, fmt.Errorf("failed to apply image store issuer: %w", err)
	}

	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	cert.SetName(imageStoreName)
	cert.SetNamespace(namespace.Namespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cert, func() error {
		cert.SetLabels(labels)
		host := imageStoreService + "." + namespace.Namespace
		secretLabels := map[string]any{}
		for k, v := range labels {
			secretLabels[k] = v
		}
		return unstructured.SetNestedField(cert.Object, map[string]any{
			"secretName":     imageStoreSecret,
			"secretTemplate": map[string]any{"labels": secretLabels},
			"commonName":     host + ".svc.cluster.local",
			"dnsNames":       []any{imageStoreService, host, host + ".svc", host + ".svc.cluster.local"},
			"duration":       "87600h",
			"issuerRef":      map[string]any{"name": imageStoreName, "kind": "Issuer"},
		}, "spec")
	}); err != nil {
		return nil, fmt.Errorf("failed to apply image store certificate: %w", err)
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreSecret}, secret); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	ca := secret.Data[imageStoreCAKey]
	if len(ca) == 0 {
		return nil, nil
	}

	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ImageStoreCA, Namespace: namespace.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Labels = labels
		cm.Data = map[string]string{imageStoreCAKey: string(ca)}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to apply image store CA: %w", err)
	}
	return &v1alpha1.ConfigMapKey{Name: ImageStoreCA, Key: imageStoreCAKey}, nil
}

// reconcileImageStore runs the image store on every node behind the
// "registry" Service and reports whether it is ready.
func (r *EnvironmentReconciler) reconcileImageStore(ctx context.Context, env *v1alpha1.Environment) v1alpha1.ComponentStatus {
	st := v1alpha1.ComponentStatus{Name: componentImageStore}
	if r.ImageStoreImage == "" {
		return failed(st, fmt.Errorf("the operator image is unknown, cannot run the image store"))
	}
	labels := imageStoreLabels(env.Name)

	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreService}, svc)
	if err == nil && !isOwnedBy(svc.Labels, env.Name) {
		return failed(st, fmt.Errorf("service %s/%s exists and is not owned by the environment (a local Docker registry?); remove it to use the image store", namespace.Namespace, imageStoreService))
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return failed(st, err)
	}

	socket := defaultSocketPath
	if env.Spec.ImageStore.ContainerdSocket != "" {
		socket = env.Spec.ImageStore.ContainerdSocket
	}
	selector := map[string]string{"app.kubernetes.io/name": imageStoreName}
	podLabels := map[string]string{"app.kubernetes.io/name": imageStoreName}
	socketType := corev1.HostPathSocket
	root := int64(0)
	readOnly := true

	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: imageStoreName, Namespace: namespace.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ds, func() error {
		ds.Labels = labels
		ds.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
		ds.Spec.Template.Labels = podLabels
		ds.Spec.Template.Spec = corev1.PodSpec{
			// Runs on every node, including tainted engine nodes.
			Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			Containers: []corev1.Container{{
				Name:    "imagestore",
				Image:   r.ImageStoreImage,
				Command: []string{"/overlock-imagestore", "--containerd-socket=/run/containerd/containerd.sock"},
				Ports:   []corev1.ContainerPort{{Name: "https", ContainerPort: imageStorePort}},
				ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
					Path: "/v2/", Port: intstr.FromInt(imageStorePort), Scheme: corev1.URISchemeHTTPS,
				}}},
				// Root is needed to use the containerd socket of the node.
				SecurityContext: &corev1.SecurityContext{RunAsUser: &root, ReadOnlyRootFilesystem: &readOnly},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "containerd", MountPath: "/run/containerd/containerd.sock"},
					{Name: "tls", MountPath: "/tls", ReadOnly: true},
				},
			}},
			Volumes: []corev1.Volume{
				{Name: "containerd", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: socket, Type: &socketType}}},
				{Name: "tls", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: imageStoreSecret}}},
			},
		}
		return nil
	}); err != nil {
		return failed(st, fmt.Errorf("failed to apply image store: %w", err))
	}

	local := corev1.ServiceInternalTrafficPolicyLocal
	svc = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: imageStoreService, Namespace: namespace.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = labels
		svc.Spec.Selector = selector
		// Clients read the store of their own node, which holds the
		// images loaded into that node.
		svc.Spec.InternalTrafficPolicy = &local
		svc.Spec.Ports = []corev1.ServicePort{{Name: "https", Port: 443, TargetPort: intstr.FromInt(imageStorePort)}}
		return nil
	}); err != nil {
		return failed(st, fmt.Errorf("failed to apply image store service: %w", err))
	}

	st.State = v1alpha1.ComponentInstalled
	st.Owned = true
	st.Ready = ds.Status.DesiredNumberScheduled > 0 && ds.Status.NumberReady == ds.Status.DesiredNumberScheduled
	if !st.Ready {
		st.Message = fmt.Sprintf("%d of %d nodes ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
	}
	return st
}

// removeImageStore deletes the image store objects owned by env.
func (r *EnvironmentReconciler) removeImageStore(ctx context.Context, env *v1alpha1.Environment) error {
	owned := func(obj client.Object) error {
		if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if apierrors.IsNotFound(err) || isNoMatch(err) {
				return nil
			}
			return err
		}
		if !isOwnedBy(obj.GetLabels(), env.Name) {
			return nil
		}
		return client.IgnoreNotFound(r.Delete(ctx, obj))
	}
	key := func(obj client.Object, name string) client.Object {
		obj.SetName(name)
		obj.SetNamespace(namespace.Namespace)
		return obj
	}
	cert, issuer := &unstructured.Unstructured{}, &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	issuer.SetGroupVersionKind(issuerGVK)
	for _, obj := range []client.Object{
		key(&appsv1.DaemonSet{}, imageStoreName),
		key(&corev1.Service{}, imageStoreService),
		key(&corev1.ConfigMap{}, ImageStoreCA),
		key(cert, imageStoreName),
		key(issuer, imageStoreName),
		key(&corev1.Secret{}, imageStoreSecret),
	} {
		if err := owned(obj); err != nil {
			return fmt.Errorf("failed to remove image store %T %s: %w", obj, obj.GetName(), err)
		}
	}
	return nil
}
