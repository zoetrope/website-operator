package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
)

// WebSiteReconciler reconciles a WebSite object
type WebSiteReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites/status,verbs=get;update;patch

func (r *WebSiteReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("website", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *WebSiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&websitev1beta1.WebSite{}).
		Complete(r)
}
