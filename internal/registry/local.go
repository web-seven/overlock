package registry

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/namespace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	deployName = "kndp-registry"
	svcName    = "registry"
)

// Create in cluster registry
func (r *Registry) CreateLocal(ctx context.Context, client *kubernetes.Clientset) error {

	deploy := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name: deployName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deployName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"app": deployName,
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

	_, err := deployments.Get(ctx, deploy.GetName(), v1.GetOptions{})

	if err == nil {
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
			Name: svcName,
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
	_, err = svcs.Get(ctx, svc.GetName(), v1.GetOptions{})
	if err == nil {
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
func (r *Registry) DeleteLocal(ctx context.Context, client *kubernetes.Clientset, logger *log.Logger) error {
	svcs := client.CoreV1().Services(namespace.Namespace)
	eSvc, _ := svcs.Get(ctx, svcName, v1.GetOptions{})
	if eSvc != nil {
		err := svcs.Delete(ctx, svcName, v1.DeleteOptions{})
		if err != nil {
			return err
		}
	} else {
		logger.Warnf("Service %s not found", svcName)
	}
	deployments := client.AppsV1().Deployments(namespace.Namespace)
	eDeploy, _ := deployments.Get(ctx, deployName, v1.GetOptions{})
	if eDeploy != nil {
		err := deployments.Delete(ctx, deployName, v1.DeleteOptions{})
		if err != nil {
			return err
		}
	} else {
		logger.Warnf("Deployment %s not found", deployName)
	}
	return nil
}
