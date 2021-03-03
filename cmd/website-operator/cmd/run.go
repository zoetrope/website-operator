package cmd

import (
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	"github.com/zoetrope/website-operator/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(websitev1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func subMain() error {
	opts := zap.Options{
		Development: config.development,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     config.metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: config.probeAddr,
		LeaderElection:         config.enableLeaderElection,
		LeaderElectionID:       config.leaderElectionID,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	if err = controllers.NewWebSiteReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("WebSite"),
		mgr.GetScheme(),
		config.nginxContainerImage,
		config.repoCheckerContainerImage,
		os.Getenv("POD_NAMESPACE"),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WebSite")
		return err
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
