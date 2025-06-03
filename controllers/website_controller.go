package controllers

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/zoetrope/website-operator"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

const (
	OperatorName              = "website-operator"
	AppNameBuildScript        = "build-script"
	AppNameRepoChecker        = "repo-checker"
	AppNameRepoCheckerService = "repo-checker-service"
	AppNameNginx              = "nginx"
	AppNameNginxService       = "nginx-service"
	ManagedByKey              = "app.kubernetes.io/managed-by"
	AppNameKey                = "app.kubernetes.io/name"
	InstanceKey               = "app.kubernetes.io/instance"
	RepoCheckerPort           = 9090
	RepoCheckerSuffix         = "-repo-checker"
	BuildScriptName           = "build"
	AfterBuildScriptName      = "after-build"
	NginxPort                 = 8080
	AnnChecksumConfig         = "checksum/config"
)

func NewWebSiteReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme, nginxContainerImage string, repoCheckerContainerImage string, operatorNamespace string, revCli RevisionClient) *WebSiteReconciler {
	return &WebSiteReconciler{
		client:                    client,
		log:                       log,
		scheme:                    scheme,
		nginxContainerImage:       nginxContainerImage,
		repoCheckerContainerImage: repoCheckerContainerImage,
		operatorNamespace:         operatorNamespace,
		revisionClient:            revCli,
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
	revisionClient            RevisionClient
}

//+kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=website.zoetrope.github.io,resources=websites/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services/status,verbs=get
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="batch",resources=jobs/status,verbs=get

func (r *WebSiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	if errors.Is(err, errJobIsActive) {
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
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

	if isUpdatedAtLeastOnce || webSite.Status.Ready == corev1.ConditionFalse {
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

	isUpdated, buildScriptHash, err := r.reconcileScriptConfigMap(ctx, webSite, &webSite.Spec.BuildScript, BuildScriptName)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create ConfigMap for build script")
		return isUpdatedAtLeastOnce, "", err
	}

	isUpdated, afterBuildScriptHash, err := r.reconcileScriptConfigMap(ctx, webSite, webSite.Spec.AfterBuildScript, AfterBuildScriptName)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create ConfigMap for after build script")
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

	revision, err := r.revisionClient.GetLatestRevision(ctx, webSite)
	if err != nil {
		log.Error(err, "failed to get revision from RepoChecker")
		return isUpdatedAtLeastOnce, "", err
	}

	isUpdated, err = r.reconcileNginxDeployment(ctx, webSite, revision, buildScriptHash)
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

	isUpdated, err = r.reconcileExtraResources(ctx, webSite)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create extraResources")
		return isUpdatedAtLeastOnce, "", err
	}

	isUpdated, err = r.reconcileAfterBuildScript(ctx, webSite, revision, afterBuildScriptHash)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err == errJobIsActive {
		return isUpdatedAtLeastOnce, "", err
	}
	if err != nil {
		log.Error(err, "failed to create Job for AfterBuildScript")
		return isUpdatedAtLeastOnce, "", err
	}

	return isUpdatedAtLeastOnce, revision, nil
}

func (r *WebSiteReconciler) reconcileScriptConfigMap(ctx context.Context, webSite *websitev1beta1.WebSite, source *websitev1beta1.DataSource, scriptType string) (bool, string, error) {
	log := r.log.WithValues("website", webSite.Name)

	if source == nil {
		return false, "", nil
	}

	script := ""
	if source.RawData != nil {
		script = *source.RawData
	} else if source.ConfigMap != nil {
		buildScriptConfigMap := &corev1.ConfigMap{}
		ns := r.operatorNamespace
		if len((*source.ConfigMap).Namespace) != 0 {
			ns = (*source.ConfigMap).Namespace
		}
		err := r.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: (*source.ConfigMap).Name}, buildScriptConfigMap)
		if err != nil {
			return false, "", err
		}
		var ok bool
		script, ok = buildScriptConfigMap.Data[(*source.ConfigMap).Key]
		if !ok {
			return false, "", fmt.Errorf("ConfigMap %s:%s does not have %s", ns, (*source.ConfigMap).Name, (*source.ConfigMap).Key)
		}
	} else {
		return false, "", errors.New("buildScript should not be empty")
	}

	cm := &corev1.ConfigMap{}
	cm.SetNamespace(webSite.Namespace)
	cm.SetName(webSite.Name + "-" + scriptType + "-script")
	hash := fmt.Sprintf("%x", md5.Sum([]byte(script)))

	op, err := ctrl.CreateOrUpdate(ctx, r.client, cm, func() error {
		setStandardLabels(AppNameBuildScript, &cm.ObjectMeta)
		if cm.Annotations == nil {
			cm.Annotations = make(map[string]string)
		}
		cm.Annotations[AnnChecksumConfig] = hash

		cm.Data = map[string]string{
			scriptType + ".sh": script,
		}
		return ctrl.SetControllerReference(webSite, cm, r.scheme)
	})

	if err != nil {
		log.Error(err, "unable to reconcile build script configmap")
		return false, hash, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info("reconcile build script configmap successfully", "op", op)
		return true, hash, nil
	}
	return false, hash, nil
}

func (r *WebSiteReconciler) reconcileRepoCheckerDeployment(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)

	deployment := &appsv1.Deployment{}
	deployment.SetNamespace(webSite.Namespace)
	deployment.SetName(webSite.Name + RepoCheckerSuffix)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, deployment, func() error {
		setStandardLabels(AppNameRepoChecker, &deployment.ObjectMeta)
		deployment.Spec.Replicas = pointer.Int32Ptr(1)
		deployment.Spec.Selector = &metav1.LabelSelector{}
		if deployment.Spec.Selector.MatchLabels == nil {
			deployment.Spec.Selector.MatchLabels = make(map[string]string)
		}
		deployment.Spec.Selector.MatchLabels[ManagedByKey] = OperatorName
		deployment.Spec.Selector.MatchLabels[AppNameKey] = AppNameRepoChecker

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
	newTemplate.Annotations = make(map[string]string)
	if webSite.Spec.PodTemplate != nil {
		for k, v := range webSite.Spec.PodTemplate.Labels {
			newTemplate.Labels[k] = v
		}
		for k, v := range webSite.Spec.PodTemplate.Annotations {
			newTemplate.Annotations[k] = v
		}
	}
	newTemplate.Labels[ManagedByKey] = OperatorName
	newTemplate.Labels[AppNameKey] = AppNameRepoChecker
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
				Value: "/var/ubuntu",
			},
		),
	}

	newTemplate.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsUser: pointer.Int64Ptr(10000),
		FSGroup:   pointer.Int64Ptr(10000),
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
	for _, secret := range webSite.Spec.ImagePullSecrets {
		newTemplate.Spec.ImagePullSecrets = append(newTemplate.Spec.ImagePullSecrets, secret)
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
			Name:  "RESOURCE_NAMESPACE",
			Value: webSite.Namespace,
		},
		{
			Name:  "RESOURCE_NAME",
			Value: webSite.Name,
		},
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
		setStandardLabels(AppNameRepoCheckerService, &service.ObjectMeta)
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
		service.Spec.Selector[AppNameKey] = AppNameRepoChecker
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

func (r *WebSiteReconciler) reconcileNginxDeployment(ctx context.Context, webSite *websitev1beta1.WebSite, revision string, hash string) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)
	deployment := &appsv1.Deployment{}
	deployment.SetNamespace(webSite.Namespace)
	deployment.SetName(webSite.Name)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, deployment, func() error {
		setStandardLabels(AppNameNginx, &deployment.ObjectMeta)
		deployment.Spec.Replicas = &webSite.Spec.Replicas
		deployment.Spec.Selector = &metav1.LabelSelector{}
		if deployment.Spec.Selector.MatchLabels == nil {
			deployment.Spec.Selector.MatchLabels = make(map[string]string)
		}
		deployment.Spec.Selector.MatchLabels[ManagedByKey] = OperatorName
		deployment.Spec.Selector.MatchLabels[AppNameKey] = AppNameNginx

		podTemplate, err := r.makeNginxPodTemplate(ctx, webSite, revision, hash)
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

func (r *WebSiteReconciler) makeNginxPodTemplate(ctx context.Context, webSite *websitev1beta1.WebSite, revision string, hash string) (*corev1.PodTemplateSpec, error) {
	newTemplate := corev1.PodTemplateSpec{}
	newResourceRequirements := corev1.ResourceRequirements{}

	newTemplate.Labels = make(map[string]string)
	newTemplate.Annotations = make(map[string]string)
	if webSite.Spec.PodTemplate != nil {
		for k, v := range webSite.Spec.PodTemplate.Labels {
			newTemplate.Labels[k] = v
		}
		for k, v := range webSite.Spec.PodTemplate.Annotations {
			newTemplate.Annotations[k] = v
		}
		newResourceRequirements = webSite.Spec.PodTemplate.NginxContainerResources
	}

	newTemplate.Labels[ManagedByKey] = OperatorName
	newTemplate.Labels[AppNameKey] = AppNameNginx
	newTemplate.Labels[InstanceKey] = webSite.Name
	newTemplate.Annotations[AnnChecksumConfig] = hash

	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes, getVolumeOrEmptyDir(webSite, "data"))
	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes, getVolumeOrEmptyDir(webSite, "log"))
	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes, getVolumeOrEmptyDir(webSite, "cache"))
	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes, getVolumeOrEmptyDir(webSite, "tmp"))
	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes, getVolumeOrEmptyDir(webSite, "home"))

	newTemplate.Spec.Volumes = append(newTemplate.Spec.Volumes,
		corev1.Volume{
			Name: BuildScriptName + "-script",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: webSite.Name + "-" + BuildScriptName + "-script",
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
	newTemplate.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup: pointer.Int64Ptr(10000),
	}

	newTemplate.Spec.Containers = append(newTemplate.Spec.Containers, corev1.Container{
		Name:  "nginx",
		Image: r.nginxContainerImage,
		Resources: newResourceRequirements,
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
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:        "/",
					Port:        intstr.FromInt(8080),
					HTTPHeaders: nil,
				},
			},
			TimeoutSeconds:   1,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		},
	})

	// create init containers and append them to Pod
	buildContainer := corev1.Container{
		Name:    "build",
		Image:   webSite.Spec.BuildImage,
		Command: []string{"/bin/bash", "-c", "/build/" + BuildScriptName + ".sh"},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64Ptr(10000),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/home/ubuntu",
				Name:      "home",
			},
			{
				MountPath: "/build",
				Name:      BuildScriptName + "-script",
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
				Value: revision,
			},
			corev1.EnvVar{
				Name:  "OUTPUT",
				Value: "/data",
			},
		),
	}

	if webSite.Spec.DeployKeySecretName != nil {
		buildContainer.VolumeMounts = append(buildContainer.VolumeMounts, corev1.VolumeMount{
			MountPath: "/home/ubuntu/.ssh",
			Name:      "deploy-key",
		})
	}

	for _, secret := range webSite.Spec.BuildSecrets {
		buildContainer.Env = append(buildContainer.Env, corev1.EnvVar{
			Name: secret.Key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
					Key: secret.Key,
				},
			},
		})
	}
	for _, secret := range webSite.Spec.ImagePullSecrets {
		newTemplate.Spec.ImagePullSecrets = append(newTemplate.Spec.ImagePullSecrets, secret)
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
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		if webSite.Spec.ServiceTemplate != nil {
			for k, v := range webSite.Spec.ServiceTemplate.Labels {
				service.Labels[k] = v
			}
			for k, v := range webSite.Spec.ServiceTemplate.Annotations {
				service.Annotations[k] = v
			}
		}
		setStandardLabels(AppNameNginxService, &service.ObjectMeta)
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
		service.Spec.Selector[AppNameKey] = AppNameNginx
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

func (r *WebSiteReconciler) reconcileExtraResources(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)
	isUpdated := false

	for _, res := range webSite.Spec.ExtraResources {
		extra, err := r.extraResource(ctx, webSite, &res)
		if err != nil {
			return false, err
		}
		obj := extra.DeepCopy()
		op, err := ctrl.CreateOrUpdate(ctx, r.client, obj, func() error {
			if !equality.Semantic.DeepDerivative(extra, obj) {
				obj = extra.DeepCopy()
			}
			return ctrl.SetControllerReference(webSite, obj, r.scheme)
		})
		if err != nil {
			return false, err
		}
		if op != controllerutil.OperationResultNone {
			log.Info("reconcile extraResource successfully", "op", op)
			isUpdated = true
		}
	}

	return isUpdated, nil
}

func (r *WebSiteReconciler) extraResource(ctx context.Context, webSite *websitev1beta1.WebSite, res *websitev1beta1.DataSource) (*unstructured.Unstructured, error) {
	resourceTemplate := ""
	if res.RawData != nil {
		resourceTemplate = *res.RawData
	} else if res.ConfigMap != nil {
		resourceTemplateConfigMap := &corev1.ConfigMap{}
		ns := r.operatorNamespace
		if len((*res.ConfigMap).Namespace) != 0 {
			ns = (*res.ConfigMap).Namespace
		}
		err := r.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: (*res.ConfigMap).Name}, resourceTemplateConfigMap)
		if err != nil {
			return nil, err
		}
		var ok bool
		resourceTemplate, ok = resourceTemplateConfigMap.Data[(*res.ConfigMap).Key]
		if !ok {
			return nil, fmt.Errorf("ConfigMap %s:%s does not have %s", ns, (*res.ConfigMap).Name, (*res.ConfigMap).Key)
		}
	} else {
		return nil, errors.New("extraResource should not be empty")
	}
	t, err := template.New("extra").Parse(resourceTemplate)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	err = t.Execute(buf, struct {
		ResourceName      string
		ResourceNamespace string
	}{
		ResourceName:      webSite.Name,
		ResourceNamespace: webSite.Namespace,
	})
	if err != nil {
		return nil, err
	}
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(buf.Bytes(), &obj)
	if err != nil {
		return nil, err
	}
	obj.SetNamespace(webSite.Namespace)
	return &obj, nil
}

var errJobIsActive = errors.New("job is active")

func (r *WebSiteReconciler) reconcileAfterBuildScript(ctx context.Context, webSite *websitev1beta1.WebSite, revision string, hash string) (bool, error) {
	if webSite.Spec.AfterBuildScript == nil {
		return false, nil
	}

	log := r.log.WithValues("website", webSite.Name)

	job := &batchv1.Job{}
	job.SetNamespace(webSite.Namespace)
	job.SetName(webSite.Name)
	template := corev1.PodTemplateSpec{}
	template.Annotations = make(map[string]string)
	template.Annotations[AnnChecksumConfig] = hash
	template.Spec.RestartPolicy = corev1.RestartPolicyNever

	template.Spec.Volumes = append(template.Spec.Volumes, getVolumeOrEmptyDir(webSite, "log"))
	template.Spec.Volumes = append(template.Spec.Volumes, getVolumeOrEmptyDir(webSite, "cache"))
	template.Spec.Volumes = append(template.Spec.Volumes, getVolumeOrEmptyDir(webSite, "tmp"))
	template.Spec.Volumes = append(template.Spec.Volumes,
		corev1.Volume{
			Name: AfterBuildScriptName + "-script",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: webSite.Name + "-" + AfterBuildScriptName + "-script",
					},
					DefaultMode: pointer.Int32Ptr(0755),
				},
			},
		},
	)

	if webSite.Spec.DeployKeySecretName != nil {
		template.Spec.Volumes = append(template.Spec.Volumes,
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
	template.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup: pointer.Int64Ptr(10000),
	}
	buildContainer := corev1.Container{
		Name:    "job",
		Image:   webSite.Spec.BuildImage,
		Command: []string{"/bin/bash", "-c", "/after-build/" + AfterBuildScriptName + ".sh"},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: pointer.Int64Ptr(10000),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/after-build",
				Name:      AfterBuildScriptName + "-script",
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
				Value: revision,
			},
		),
	}
	if webSite.Spec.DeployKeySecretName != nil {
		buildContainer.VolumeMounts = append(buildContainer.VolumeMounts, corev1.VolumeMount{
			MountPath: "/home/ubuntu/.ssh",
			Name:      "deploy-key",
		})
	}
	for _, secret := range webSite.Spec.BuildSecrets {
		buildContainer.Env = append(buildContainer.Env, corev1.EnvVar{
			Name: secret.Key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
					Key: secret.Key,
				},
			},
		})
	}
	for _, secret := range webSite.Spec.ImagePullSecrets {
		template.Spec.ImagePullSecrets = append(template.Spec.ImagePullSecrets, secret)
	}
	template.Spec.Containers = append(template.Spec.Containers, buildContainer)

	err := r.client.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Name}, job)
	if err == nil {
		if job.Status.Active != 0 {
			return false, errJobIsActive
		}
		if equality.Semantic.DeepDerivative(template, job.Spec.Template) {
			return false, nil
		}
		propergationPolicy := metav1.DeletePropagationBackground
		err = r.client.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propergationPolicy})
		if err != nil {
			return false, err
		}
	} else if !apierrors.IsNotFound(err) {
		return false, err
	}

	newJob := &batchv1.Job{}
	newJob.SetNamespace(webSite.Namespace)
	newJob.SetName(webSite.Name)
	newJob.Spec.Template = template
	err = ctrl.SetControllerReference(webSite, newJob, r.scheme)
	if err != nil {
		return false, err
	}

	err = r.client.Create(ctx, newJob, &client.CreateOptions{})
	if err != nil {
		log.Error(err, "unable to reconcile after build script job")
		return false, err
	}

	log.Info("reconcile Job for AfterBuildScript successfully")
	return true, nil
}

func setStandardLabels(app string, om *metav1.ObjectMeta) {
	if om.Labels == nil {
		om.Labels = make(map[string]string)
	}
	om.Labels[ManagedByKey] = OperatorName
	om.Labels[AppNameKey] = app
}

func selectReadyWebSite(obj client.Object) []string {
	site := obj.(*websitev1beta1.WebSite)
	return []string{string(site.Status.Ready)}
}

func getVolumeOrEmptyDir(webSite *websitev1beta1.WebSite, name string) corev1.Volume {
	for _, v := range webSite.Spec.VolumeTemplates {
		if v.Name == name {
			return v
		}
	}
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebSiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &websitev1beta1.WebSite{}, website.WebSiteIndexField, selectReadyWebSite)
	if err != nil {
		return err
	}

	ch := make(chan event.TypedGenericEvent[*websitev1beta1.WebSite])
	watcher := newRevisionWatcher(mgr.GetClient(), mgr.GetLogger().WithName("RevisionWatcher"), ch, 1*time.Minute, r.revisionClient)
	err = mgr.Add(watcher)
	if err != nil {
		return err
	}

	logger := mgr.GetLogger().WithName("ConfigMap Handler")
	cmHandler := func(ctx context.Context, o client.Object) []reconcile.Request {
		wsl := &websitev1beta1.WebSiteList{}
		err := r.client.List(ctx, wsl)
		if err != nil {
			logger.Error(err, "failed to list WebSites")
			return nil
		}
		var requests []reconcile.Request
		for _, ws := range wsl.Items {
			if ws.Spec.BuildScript.ConfigMap != nil && ws.Spec.BuildScript.ConfigMap.Name == o.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: ws.GetNamespace(),
						Name:      ws.GetName(),
					},
				})
				continue
			}
			if ws.Spec.AfterBuildScript != nil &&
				ws.Spec.AfterBuildScript.ConfigMap != nil &&
				ws.Spec.AfterBuildScript.ConfigMap.Name == o.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: ws.GetNamespace(),
						Name:      ws.GetName(),
					},
				})
				continue
			}
		}
		return requests
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&websitev1beta1.WebSite{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&batchv1.Job{}).
		WatchesRawSource(source.Channel(ch, &handler.TypedEnqueueRequestForObject[*websitev1beta1.WebSite]{})).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(cmHandler)).
		Complete(r)
}
