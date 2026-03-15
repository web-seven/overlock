package registry

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/web-seven/overlock/internal/certmanager"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/namespace"
	"github.com/web-seven/overlock/internal/policy"
)

const (
	deployName           = "overlock-registry"
	svcName              = "registry"
	deployPort           = 5000
	nginxPortHTTP        = 80
	nginxPortHTTPS       = 443
	svcPort              = 80
	svcPortTLS           = 443
	nodePort             = 30100
	tlsCertPath          = "/certs/tls.crt"
	tlsKeyPath           = "/certs/tls.key"
	tlsVolumeName        = "registry-tls"
	tlsMountPath         = "/certs"
	configVolumeName     = "registry-config"
	configMountPath      = "/etc/docker/registry"
	configMapName        = "registry-config"
	nginxConfigMapName   = "nginx-proxy-config"
	nginxConfigMountPath = "/etc/nginx/conf.d"
)

var (
	matchLabels = map[string]string{
		"app": deployName,
	}
)

type RegistryReconciler struct {
	client.Client
	context.CancelFunc
}

// Create in cluster registry
func (r *Registry) CreateLocal(ctx context.Context, client *kubernetes.Clientset, logger *zap.SugaredLogger) error {
	configClient, err := config.GetConfigWithContext(r.Context)
	if err != nil {
		return err
	}

	// Install cert-manager and create TLS certificate
	logger.Debug("Installing cert-manager")
	if err := certmanager.InstallCertManager(ctx, configClient, nil); err != nil {
		logger.Warnf("Failed to install cert-manager: %v", err)
	} else {
		logger.Debug("cert-manager installed")

		logger.Debug("Creating self-signed issuer")
		if err := certmanager.CreateSelfSignedIssuer(ctx, configClient); err != nil {
			logger.Warnf("Failed to create self-signed issuer: %v", err)
		}
	}

	// Create namespace first so we can create the certificate
	err = namespace.CreateNamespace(ctx, configClient)
	if err != nil {
		return err
	}

	// Create registry TLS certificate
	logger.Debug("Creating registry TLS certificate")
	if err := certmanager.CreateRegistryCertificate(ctx, configClient); err != nil {
		logger.Warnf("Failed to create registry certificate: %v", err)
	}

	// Create ConfigMap with registry configuration (HTTP only, nginx handles TLS)
	registryConfig := `version: 0.1
log:
  fields:
    service: registry
storage:
  filesystem:
    rootdirectory: /var/lib/registry
  delete:
    enabled: true
http:
  addr: :5000
`
	configMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace.Namespace,
		},
		Data: map[string]string{
			"config.yml": registryConfig,
		},
	}

	// Create nginx reverse proxy ConfigMap for HTTP and HTTPS
	nginxConfig := `server {
    listen 80;
    server_name _;
    client_max_body_size 0;

    location / {
        proxy_pass http://localhost:5000;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 900;
    }
}

server {
    listen 443 ssl;
    server_name _;
    client_max_body_size 0;

    ssl_certificate /certs/tls.crt;
    ssl_certificate_key /certs/tls.key;

    location / {
        proxy_pass http://localhost:5000;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 900;
    }
}
`
	nginxConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      nginxConfigMapName,
			Namespace: namespace.Namespace,
		},
		Data: map[string]string{
			"default.conf": nginxConfig,
		},
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      deployName,
			Namespace: namespace.Namespace,
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configVolumeName,
									MountPath: configMountPath,
									ReadOnly:  true,
								},
							},
						},
						{
							Name:  "nginx",
							Image: "nginx:alpine",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: nginxPortHTTP,
								},
								{
									Name:          "https",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: nginxPortHTTPS,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      tlsVolumeName,
									MountPath: tlsMountPath,
									ReadOnly:  true,
								},
								{
									Name:      "nginx-config",
									MountPath: nginxConfigMountPath,
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: tlsVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: certmanager.GetRegistrySecretName(),
								},
							},
						},
						{
							Name: configVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
						{
							Name: "nginx-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: nginxConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      svcName,
			Namespace: namespace.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     "NodePort",
			Selector: deploy.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       svcPort,
					TargetPort: intstr.FromInt(nginxPortHTTP),
				},
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       svcPortTLS,
					TargetPort: intstr.FromInt(nginxPortHTTPS),
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)
	ctrlClient, _ := ctrl.New(configClient, ctrl.Options{Scheme: scheme})
	for _, res := range []ctrl.Object{configMap, nginxConfigMap, deploy, svc} {
		_, err := controllerutil.CreateOrUpdate(ctx, ctrlClient, res, func() error { return nil })
		if err != nil {
			return err
		}
	}

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	deployIsReady := false
	for !deployIsReady {
		select {
		case <-timeout:
			return errors.New("local registry to not comes ready")
		case <-ticker.C:
			deploy, err = client.AppsV1().
				Deployments(namespace.Namespace).
				Get(ctx, deploy.GetName(), v1.GetOptions{})
			if err != nil {
				return err
			}
			deployIsReady = deploy.Status.ReadyReplicas > 0

			if deployIsReady {
				logger.Debug("Installing policy controller")
				err = policy.AddPolicyConroller(ctx, configClient, policy.DefaultPolicyController, nil)
				if err != nil {
					logger.Warnln("Policy controller has issues, without it, local registry could not work normally.")
					return err
				}
				logger.Debug("Policy controller installed.")

				logger.Debug("Installing policies")
				err = policy.AddRegistryPolicy(ctx,
					configClient,
					&policy.RegistryPolicy{
						Name:     r.Name,
						Url:      r.Server,
						NodePort: fmt.Sprintf("%v", svc.Spec.Ports[0].NodePort),
					},
				)
				if err != nil {
					return err
				}
				logger.Debug("Policies installed.")
			}
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

func IsLocalRegistry(ctx context.Context, client *kubernetes.Clientset) (bool, error) {
	pods := client.CoreV1().Pods(namespace.Namespace)
	regs, err := pods.List(ctx, v1.ListOptions{Limit: 1, LabelSelector: "app=" + deployName})
	if err != nil {
		return false, err
	}
	if len(regs.Items) == 0 {
		return false, fmt.Errorf("not found local registry")
	}
	return true, nil
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
			logger.Error(err)
			close(stopChan)
			return
		}
		// Use insecure transport for self-signed certificate
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		err = remote.Write(ref, image, remote.WithTransport(transport))
		if err != nil {
			logger.Error(err)
			close(stopChan)
			return
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
			defer func() { _ = l.Close() }()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// ListLocalRegistryTags lists all tags for an image in the local registry
func ListLocalRegistryTags(ctx context.Context, imageName string, config *rest.Config, logger *zap.SugaredLogger) ([]string, error) {
	client, err := kube.Client(config)
	if err != nil {
		return nil, err
	}

	pods := client.CoreV1().Pods(namespace.Namespace)
	regs, err := pods.List(ctx, v1.ListOptions{Limit: 1, LabelSelector: "app=" + deployName})
	if err != nil {
		return nil, err
	}

	if len(regs.Items) == 0 {
		return nil, fmt.Errorf("local registry not found")
	}

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}

	lPort, err := getFreePort()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	var tags []string
	var listErr error

	go func() {
		for range readyChan {
		}
		if len(errOut.String()) != 0 {
			close(stopChan)
			return
		}
		repoName := "localhost:" + fmt.Sprint(lPort) + "/" + imageName
		logger.Debugf("Listing tags for repository: %s", repoName)
		repo, err := name.NewRepository(repoName)
		if err != nil {
			listErr = err
			close(stopChan)
			return
		}
		// Use insecure transport for self-signed certificate
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		tags, listErr = remote.List(repo, remote.WithTransport(transport))
		if listErr != nil {
			logger.Debugf("Failed to list tags: %v", listErr)
		}
		close(stopChan)
	}()

	if err = forwarder.ForwardPorts(); err != nil {
		return nil, err
	}

	return tags, listErr
}
