package cmd

import (
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	"github.com/zoetrope/website-operator/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = websitev1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}
func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		Port:               9443,
		LeaderElection:     config.enableLeaderElection,
		LeaderElectionID:   config.leaderElectionID,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	if err = (&controllers.WebSiteReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("WebSite"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WebSite")
		return err
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
