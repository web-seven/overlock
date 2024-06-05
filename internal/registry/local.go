package registry

import (
	"context"

	"github.com/kndpio/kndp/internal/namespace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// Create in cluster registry
func (r *Registry) CreateLocal(ctx context.Context, client *kubernetes.Clientset) error {

	deploy := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name: "kndp-registry",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kndp-registry",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"app": "kndp-registry",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "registry",
							Image: "registry:2",
							Ports: []corev1.ContainerPort{
								{
									Name:          "oci",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 5000,
								},
							},
						},
					},
				},
			},
		},
	}
	deployments := client.AppsV1().Deployments(namespace.Namespace)

	eDeploy, _ := deployments.Get(ctx, deploy.GetName(), v1.GetOptions{})
	if eDeploy != nil {
		_, err := deployments.Update(ctx, deploy, v1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err := deployments.Create(ctx, deploy, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: "registry",
		},
		Spec: corev1.ServiceSpec{
			Type:     "ClusterIP",
			Selector: deploy.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "oci",
					Protocol:   corev1.ProtocolTCP,
					Port:       5000,
					TargetPort: intstr.FromInt(5000),
				},
			},
		},
	}

	svcs := client.CoreV1().Services(namespace.Namespace)

	eSvc, _ := svcs.Get(ctx, svc.GetName(), v1.GetOptions{})
	if eSvc != nil {
		_, err := svcs.Update(ctx, svc, v1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err := svcs.Create(ctx, svc, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete in cluster registry
func (r *Registry) DeleteLocal() {
	// TODO: Delete deployment with docker registry
	// TODO: Delete service with local name
}
