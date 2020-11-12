package cmd

import (
	"net/http"

	"github.com/cybozu-go/well"

	"github.com/zoetrope/website-operator/ui/backend"

	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return err
	}

	err = websitev1beta1.AddToScheme(scheme)
	if err != nil {
		return err
	}

	kubeClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	rawClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	server := backend.NewAPIServer(kubeClient, rawClient)
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", server)
	s := &well.HTTPServer{
		Server: &http.Server{
			Addr:    config.listenAddr,
			Handler: mux,
		},
	}
	err = s.ListenAndServe()
	if err != nil {
		return err
	}
	return well.Wait()
}
