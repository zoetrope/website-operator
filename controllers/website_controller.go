package controllers

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	OperatorName      = "website-operator"
	AppName           = "website"
	ManagedByKey      = "app.kubernetes.io/managed-by"
	AppNameKey        = "app.kubernetes.io/name"
	InstanceKey       = "app.kubernetes.io/instance"
	RepoCheckerPort   = 9090
	RepoCheckerSuffix = "-repo-checker"
	BuildScriptSuffix = "-build-script"
	NginxPort         = 8080
)

func NewWebSiteReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme, nginxContainerImage string, repoCheckerContainerImage string, operatorNamespace string) *WebSiteReconciler {
	return &WebSiteReconciler{
		client:                    client,
		log:                       log,
		scheme:                    scheme,
		nginxContainerImage:       nginxContainerImage,
		repoCheckerContainerImage: repoCheckerContainerImage,
		operatorNamespace:         operatorNamespace,
	}
}

// WebSiteReconciler reconciles a WebSite object
type WebSiteReconciler struct {
	client                    client.Client
	log                       logr.Logger
	scheme                    *runtime.Scheme
	nginxContainerImage       string
	repoCheckerContainerImage string
	operatorNamespace         string
}

// +kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get

func (r *WebSiteReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.log.WithValues("website", req.NamespacedName)

	webSite := &websitev1beta1.WebSite{}
	if err := r.client.Get(ctx, req.NamespacedName, webSite); err != nil {
		log.Error(err, "unable to fetch WebSite", "name", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if webSite.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	isUpdatedAtLeastOnce, revision, err := r.reconcile(ctx, webSite)
	if errors.Is(err, errRevisionNotReady) {
		return ctrl.Result{
			Requeue: true,
		}, nil
	}
	if err != nil {
		webSite.Status.Ready = corev1.ConditionFalse
		webSite.Status.Revision = revision
		errUpdate := r.client.Status().Update(ctx, webSite)
		if errUpdate != nil {
			log.Error(errUpdate, "failed to status update")
		}
		return ctrl.Result{}, err
	}

	if isUpdatedAtLeastOnce {
		webSite.Status.Ready = corev1.ConditionTrue
		webSite.Status.Revision = revision
		errUpdate := r.client.Status().Update(ctx, webSite)
		if errUpdate != nil {
			log.Error(errUpdate, "failed to status update")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *WebSiteReconciler) reconcile(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, string, error) {
	log := r.log.WithValues("website", webSite.Name)

	isUpdatedAtLeastOnce := false

	isUpdated, err := r.reconcileBuildScript(ctx, webSite)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create ConfigMap")
		return isUpdatedAtLeastOnce, "", err
	}

	isUpdated, err = r.reconcileRepoCheckerDeployment(ctx, webSite)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to reconcile RepoChecker deployment")
		return isUpdatedAtLeastOnce, "", err
	}

	isUpdated, err = r.reconcileRepoCheckerService(ctx, webSite)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create or update Service For RepoChecker")
		return isUpdatedAtLeastOnce, "", err
	}

	revision, err := r.getLatestRevision(ctx, webSite)
	if err != nil {
		log.Error(err, "failed to get revision from RepoChecker")
		return isUpdatedAtLeastOnce, "", err
	}

	isUpdated, err = r.reconcileNginxDeployment(ctx, webSite, revision)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create or update Deployment For Nginx")
		return isUpdatedAtLeastOnce, revision, err
	}

	isUpdated, err = r.reconcileNginxService(ctx, webSite)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create or update Service For Nginx")
		return isUpdatedAtLeastOnce, "", err
	}

	return isUpdatedAtLeastOnce, revision, nil
}

func (r *WebSiteReconciler) reconcileBuildScript(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)

	if webSite.Spec.BuildScript.RawData == nil {
		// TODO: implement configmap
		return false, errors.New("rawData should not be empty")
	}

	cm := &corev1.ConfigMap{}
	cm.SetNamespace(webSite.Namespace)
	cm.SetName(webSite.Name + BuildScriptSuffix)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, cm, func() error {
		setLabels(&cm.ObjectMeta)
		cm.Data = map[string]string{
			"build.sh": *webSite.Spec.BuildScript.RawData,
		}
		return ctrl.SetControllerReference(webSite, cm, r.scheme)
	})

	if err != nil {
		log.Error(err, "unable to reconcile build script configmap")
		return false, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("reconcile build script configmap successfully", "op", op)
		return true, nil
	}
	return false, nil
}

func (r *WebSiteReconciler) reconcileRepoCheckerDeployment(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)

	deployment := &appsv1.Deployment{}
	deployment.SetNamespace(webSite.Namespace)
	deployment.SetName(webSite.Name + RepoCheckerSuffix)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, deployment, func() error {
		setLabels(&deployment.ObjectMeta)
		deployment.Spec.Replicas = pointer.Int32Ptr(1)
		deployment.Spec.Selector = &metav1.LabelSelector{}
		if deployment.Spec.Selector.MatchLabels == nil {
			deployment.Spec.Selector.MatchLabels = make(map[string]string)
		}
		deployment.Spec.Selector.MatchLabels[ManagedByKey] = OperatorName
		deployment.Spec.Selector.MatchLabels[AppNameKey] = AppName

		podTemplate, err := r.makePodTemplateForRepoChecker(webSite)
		if err != nil {
			return err
		}
		if !equality.Semantic.DeepDerivative(podTemplate, &deployment.Spec.Template) {
			deployment.Spec.Template = *podTemplate
		}

		return ctrl.SetControllerReference(webSite, deployment, r.scheme)
	})

	if err != nil {
		return false, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("reconcile RepoChecker deployment successfully", "op", op)
		return true, nil
	}
	return false, nil
}

func (r *WebSiteReconciler) makePodTemplateForRepoChecker(webSite *websitev1beta1.WebSite) (*corev1.PodTemplateSpec, error) {
	newTemplate := corev1.PodTemplateSpec{}

	newTemplate.Labels = make(map[string]string)
	newTemplate.Labels[ManagedByKey] = OperatorName
	newTemplate.Labels[AppNameKey] = AppName
	newTemplate.Labels[InstanceKey] = webSite.Name + RepoCheckerSuffix

	container := corev1.Container{
		Name:  "repo-checker",
		Image: r.repoCheckerContainerImage,
		Command: []string{"/repo-checker",
			fmt.Sprintf("--repo-url=%s", webSite.Spec.RepoURL),
			fmt.Sprintf("--repo-branch=%s", webSite.Spec.Branch),
			fmt.Sprintf("--listen-addr=:%d", RepoCheckerPort),
		},
		Env: append(makeEnvCommon(webSite),
			corev1.EnvVar{
				Name:  "HOME",
				Value: "/var/www",
			},
		),
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64Ptr(10000),
		},
	}

	if webSite.Spec.DeployKeySecretName != nil {
		newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes,
			corev1.Volume{
				Name: "deploy-key",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  *webSite.Spec.DeployKeySecretName,
						DefaultMode: pointer.Int32Ptr(0600),
					},
				},
			},
		)
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      "deploy-key",
				MountPath: "/home/ubuntu/.ssh",
			},
		)
	}
	newTemplate.Spec.Containers = append(newTemplate.Spec.Containers, container)
	return &newTemplate, nil
}

func makeEnvCommon(webSite *websitev1beta1.WebSite) []corev1.EnvVar {
	items := strings.Split(webSite.Spec.RepoURL, "/")
	last := items[len(items)-1]
	repoName := strings.TrimSuffix(last, ".git")

	env := []corev1.EnvVar{
		{
			Name:  "REPO_URL",
			Value: webSite.Spec.RepoURL,
		},
		{
			Name:  "REPO_NAME",
			Value: repoName,
		},
		{
			Name:  "REPO_BRANCH",
			Value: webSite.Spec.Branch,
		},
	}
	return env
}

func (r *WebSiteReconciler) reconcileRepoCheckerService(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)
	service := &corev1.Service{}
	service.SetNamespace(webSite.Namespace)
	service.SetName(webSite.Name + RepoCheckerSuffix)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, service, func() error {
		setLabels(&service.ObjectMeta)
		ports := []corev1.ServicePort{
			{
				Name:       "repo-checker",
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(RepoCheckerPort),
			},
		}
		service.Spec.Ports = ports

		service.Spec.Selector = make(map[string]string)
		service.Spec.Selector[ManagedByKey] = OperatorName
		service.Spec.Selector[AppNameKey] = AppName
		service.Spec.Selector[InstanceKey] = webSite.Name + RepoCheckerSuffix

		return ctrl.SetControllerReference(webSite, service, r.scheme)
	})

	if err != nil {
		log.Error(err, "unable to create-or-update Service For RepoChecker")
		return false, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("reconcile Service For RepoChecker successfully", "op", op)
		return true, nil
	}
	return false, nil
}

var errRevisionNotReady = errors.New("latest revision not ready")

func (r *WebSiteReconciler) getLatestRevision(ctx context.Context, webSite *websitev1beta1.WebSite) (string, error) {
	log := r.log.WithValues("website", webSite.Name)

	repoCheckerHost := fmt.Sprintf("%s%s.%s.svc.cluster.local", webSite.Name, RepoCheckerSuffix, webSite.Namespace)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("http://%s/", repoCheckerHost),
		nil,
	)
	if err != nil {
		return "", err
	}

	cli := &well.HTTPClient{Client: &http.Client{}}
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Info("repo checker is not ready")
		return "", errRevisionNotReady
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to repo check: %s", resp.Status)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	log.Info("Retrieved Revision", "revision", b)
	return string(b), nil
}

func (r *WebSiteReconciler) reconcileNginxDeployment(ctx context.Context, webSite *websitev1beta1.WebSite, revision string) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)
	deployment := &appsv1.Deployment{}
	deployment.SetNamespace(webSite.Namespace)
	deployment.SetName(webSite.Name)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, deployment, func() error {
		setLabels(&deployment.ObjectMeta)
		deployment.Spec.Replicas = pointer.Int32Ptr(2)
		deployment.Spec.Selector = &metav1.LabelSelector{}
		if deployment.Spec.Selector.MatchLabels == nil {
			deployment.Spec.Selector.MatchLabels = make(map[string]string)
		}
		deployment.Spec.Selector.MatchLabels[ManagedByKey] = OperatorName
		deployment.Spec.Selector.MatchLabels[AppNameKey] = AppName

		podTemplate, err := r.makeNginxPodTemplate(webSite, revision)
		if err != nil {
			return err
		}
		if !equality.Semantic.DeepDerivative(podTemplate, &deployment.Spec.Template) {
			deployment.Spec.Template = *podTemplate
		}

		return ctrl.SetControllerReference(webSite, deployment, r.scheme)
	})

	if err != nil {
		log.Error(err, "unable to create-or-update Deployment For Nginx")
		return false, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("reconcile Deployment For Nginx successfully", "op", op)
		return true, nil
	}
	return false, nil
}

func (r *WebSiteReconciler) makeNginxPodTemplate(webSite *websitev1beta1.WebSite, revision string) (*corev1.PodTemplateSpec, error) {
	newTemplate := corev1.PodTemplateSpec{}

	newTemplate.Labels = make(map[string]string)
	newTemplate.Labels[ManagedByKey] = OperatorName
	newTemplate.Labels[AppNameKey] = AppName
	newTemplate.Labels[InstanceKey] = webSite.Name

	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes,
		corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "log",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "home",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: "build-script",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: webSite.Name + BuildScriptSuffix,
					},
					DefaultMode: pointer.Int32Ptr(0755),
				},
			},
		},
	)
	if webSite.Spec.DeployKeySecretName != nil {
		newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes,
			corev1.Volume{
				Name: "deploy-key",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  *webSite.Spec.DeployKeySecretName,
						DefaultMode: pointer.Int32Ptr(0600),
					},
				},
			},
		)
	}

	newTemplate.Spec.Containers = append(newTemplate.Spec.Containers, corev1.Container{
		Name:  "nginx",
		Image: r.nginxContainerImage,
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/data",
				Name:      "data",
			},
			{
				MountPath: "/var/log/nginx",
				Name:      "log",
			},
			{
				MountPath: "/var/cache/nginx",
				Name:      "cache",
			},
			{
				MountPath: "/tmp",
				Name:      "tmp",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64Ptr(33), // id for www-data
		},
	})

	// create init containers and append them to Pod
	buildContainer := corev1.Container{
		Name:    "build",
		Image:   webSite.Spec.BuildImage,
		Command: []string{"/bin/bash", "-c", "/build/build.sh"},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/home/ubuntu",
				Name:      "home",
			},
			{
				MountPath: "/build",
				Name:      "build-script",
			},
			{
				MountPath: "/data",
				Name:      "data",
			},
			{
				MountPath: "/tmp",
				Name:      "tmp",
			},
		},
		Env: append(makeEnvCommon(webSite),
			corev1.EnvVar{
				Name:  "HOME",
				Value: "/home/ubuntu",
			},
			corev1.EnvVar{
				Name:  "REVISION",
				Value: revision},
		),
	}

	if webSite.Spec.DeployKeySecretName != nil {
		buildContainer.VolumeMounts = append(buildContainer.VolumeMounts, corev1.VolumeMount{
			MountPath: "/home/ubuntu/.ssh",
			Name:      "deploy-key",
		})
	}
	newTemplate.Spec.InitContainers = append(newTemplate.Spec.InitContainers, buildContainer)
	return &newTemplate, nil
}

func (r *WebSiteReconciler) reconcileNginxService(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)
	service := &corev1.Service{}
	service.SetNamespace(webSite.Namespace)
	service.SetName(webSite.Name)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, service, func() error {
		setLabels(&service.ObjectMeta)
		ports := []corev1.ServicePort{
			{
				Name:       "nginx",
				Protocol:   corev1.ProtocolTCP,
				Port:       NginxPort,
				TargetPort: intstr.FromInt(NginxPort),
			},
		}
		service.Spec.Ports = ports

		service.Spec.Selector = make(map[string]string)
		service.Spec.Selector[ManagedByKey] = OperatorName
		service.Spec.Selector[AppNameKey] = AppName
		service.Spec.Selector[InstanceKey] = webSite.Name

		return ctrl.SetControllerReference(webSite, service, r.scheme)
	})

	if err != nil {
		log.Error(err, "unable to create-or-update Service For Nginx")
		return false, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("reconcile Service For Nginx successfully", "op", op)
		return true, nil
	}
	return false, nil
}

func setLabels(om *metav1.ObjectMeta) {
	if om.Labels == nil {
		om.Labels = make(map[string]string)
	}
	om.Labels[ManagedByKey] = OperatorName
	om.Labels[AppNameKey] = AppName
}

func (r *WebSiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&websitev1beta1.WebSite{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
