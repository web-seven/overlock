// Command overlock-operator runs the Overlock Kubernetes operator, which
// reconciles Environment resources in the cluster it is deployed to.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/go-logr/zapr"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/controller"
)

func main() {
	var metricsAddr, probeAddr string
	var leaderElect bool
	var imageStoreImage string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for the controller manager.")
	flag.StringVar(&imageStoreImage, "image-store-image", "", "Image of the image store. Defaults to the image of the operator pod.")
	opts := ctrlzap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	zapLogger := ctrlzap.NewRaw(ctrlzap.UseFlagOptions(&opts))
	ctrl.SetLogger(zapr.NewLogger(zapLogger))
	setupLog := ctrl.Log.WithName("setup")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	restConfig := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElect,
		LeaderElectionID:       "overlock-operator.overlock.app",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if imageStoreImage == "" {
		// The API may not be reachable yet, e.g. while K3s restarts; without
		// the image the image store cannot run, so retry and otherwise exit to
		// let the pod restart instead of running without it.
		ctx, cancel := context.WithTimeout(context.Background(), ownImageTimeout)
		imageStoreImage, err = ownImage(ctx, mgr.GetAPIReader(), ownImageInterval)
		cancel()
		if err != nil {
			setupLog.Error(err, "unable to get the operator image for the image store")
			os.Exit(1)
		}
	}
	if err := (&controller.EnvironmentReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Releases:        controller.NewHelmReleases(restConfig),
		ImageStoreImage: imageStoreImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", v1alpha1.EnvironmentKind)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

const (
	ownImageInterval = 2 * time.Second
	ownImageTimeout  = 2 * time.Minute
)

// ownImage returns the image of the operator pod (POD_NAME and POD_NAMESPACE
// from the downward API), which also contains the image store. It retries
// every interval until ctx is done, and returns an empty image without error
// when not running in a pod.
func ownImage(ctx context.Context, reader client.Reader, interval time.Duration) (string, error) {
	name, ns := os.Getenv("POD_NAME"), os.Getenv("POD_NAMESPACE")
	if name == "" || ns == "" {
		return "", nil
	}
	var image string
	var lastErr error
	err := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		pod := &corev1.Pod{}
		if err := reader.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, pod); err != nil {
			lastErr = err
			return false, nil
		}
		for _, container := range pod.Spec.Containers {
			if container.Name == "manager" {
				image = container.Image
			}
		}
		return true, nil
	})
	if err != nil {
		if lastErr != nil {
			return "", fmt.Errorf("cannot get operator pod %s/%s: %w", ns, name, lastErr)
		}
		return "", err
	}
	return image, nil
}
