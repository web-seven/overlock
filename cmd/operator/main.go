// Command overlock-operator runs the Overlock Kubernetes operator, which
// reconciles Environment resources in the cluster it is deployed to.
package main

import (
	"context"
	"flag"
	"os"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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
		imageStoreImage = ownImage(restConfig, scheme)
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

// ownImage returns the image of the operator pod (POD_NAME and POD_NAMESPACE
// from the downward API), which also contains the image store.
func ownImage(restConfig *rest.Config, scheme *runtime.Scheme) string {
	name, ns := os.Getenv("POD_NAME"), os.Getenv("POD_NAMESPACE")
	if name == "" || ns == "" {
		return ""
	}
	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return ""
	}
	pod := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, pod); err != nil {
		return ""
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == "manager" {
			return container.Image
		}
	}
	return ""
}
