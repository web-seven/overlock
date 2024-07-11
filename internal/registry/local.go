package registry

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/namespace"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	deployName = "kndp-registry"
	svcName    = "registry"
	deployPort = 5000
	svcPort    = 80
	nodePort   = 30100
)

var (
	matchLabels = map[string]string{
		"app": deployName,
	}
)

// Create in cluster registry
func (r *Registry) CreateLocal(ctx context.Context, client *kubernetes.Clientset) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name: deployName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: matchLabels,
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
									ContainerPort: deployPort,
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
			Type:     "NodePort",
			Selector: deploy.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "oci",
					Protocol:   corev1.ProtocolTCP,
					NodePort:   nodePort,
					Port:       svcPort,
					TargetPort: intstr.FromInt(deployPort),
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
func (r *Registry) DeleteLocal(ctx context.Context, client *kubernetes.Clientset, logger *zap.SugaredLogger) error {
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

func IsLocalRegistry(ctx context.Context, client *kubernetes.Clientset) bool {
	return true
}

func PushLocalRegistry(ctx context.Context, imageName string, image regv1.Image, config *rest.Config, logger *zap.SugaredLogger) error {

	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	pods := client.CoreV1().Pods(namespace.Namespace)
	regs, err := pods.List(ctx, v1.ListOptions{Limit: 1, LabelSelector: "app=" + deployName})
	if err != nil {
		return err
	}

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return err
	}

	lPort, err := getFreePort()
	if err != nil {
		return err
	}

	logger.Debugf("Found local registry with name: %s", regs.Items[0].GetName())

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace.Namespace, regs.Items[0].GetName())
	hostIP := strings.TrimLeft(config.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	logger.Debugf("Dialer server URL: %s", serverURL.String())

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)
	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	forwarder, err := portforward.New(dialer, []string{fmt.Sprint(lPort) + ":" + fmt.Sprint(deployPort)}, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}

	go func() {
		for range readyChan {
		}
		if len(errOut.String()) != 0 {
			close(stopChan)
		} else if len(out.String()) != 0 {
			logger.Debug(out.String())
		}
		refName := "localhost:" + fmt.Sprint(lPort) + "/" + imageName
		logger.Debugf("Try to push to reference: %s", refName)
		ref, err := name.ParseReference(refName)
		if err != nil {
			close(stopChan)
		}
		err = remote.Write(ref, image)
		if err != nil {
			close(stopChan)
		}
		logger.Debug("Pushed to remote registry.")
		close(stopChan)
	}()

	if err = forwarder.ForwardPorts(); err != nil {
		return err
	}
	return nil
}

func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}
