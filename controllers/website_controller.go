package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/zoetrope/website-operator"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
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
//+kubebuilder:rbac:groups=sebsite.zoetrope.github.io,resources=websites/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services/status,verbs=get
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get

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

	revision, err := r.revisionClient.GetLatestRevision(ctx, webSite)
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

	isUpdated, err = r.reconcileExtraResources(ctx, webSite)
	isUpdatedAtLeastOnce = isUpdatedAtLeastOnce || isUpdated
	if err != nil {
		log.Error(err, "failed to create extraResources")
		return isUpdatedAtLeastOnce, "", err
	}

	return isUpdatedAtLeastOnce, revision, nil
}

func (r *WebSiteReconciler) reconcileBuildScript(ctx context.Context, webSite *websitev1beta1.WebSite) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)

	buildScript := ""
	if webSite.Spec.BuildScript.RawData != nil {
		buildScript = *webSite.Spec.BuildScript.RawData
	} else if webSite.Spec.BuildScript.ConfigMap != nil {
		buildScriptConfigMap := &corev1.ConfigMap{}
		ns := r.operatorNamespace
		if len((*webSite.Spec.BuildScript.ConfigMap).Namespace) != 0 {
			ns = (*webSite.Spec.BuildScript.ConfigMap).Namespace
		}
		err := r.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: (*webSite.Spec.BuildScript.ConfigMap).Name}, buildScriptConfigMap)
		if err != nil {
			return false, err
		}
		var ok bool
		buildScript, ok = buildScriptConfigMap.Data[(*webSite.Spec.BuildScript.ConfigMap).Key]
		if !ok {
			return false, fmt.Errorf("ConfigMap %s:%s does not have %s", ns, (*webSite.Spec.BuildScript.ConfigMap).Name, (*webSite.Spec.BuildScript.ConfigMap).Key)
		}
	} else {
		return false, errors.New("buildScript should not be empty")
	}

	cm := &corev1.ConfigMap{}
	cm.SetNamespace(webSite.Namespace)
	cm.SetName(webSite.Name + BuildScriptSuffix)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, cm, func() error {
		setStandardLabels(&cm.ObjectMeta)
		cm.Data = map[string]string{
			"build.sh": buildScript,
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
		setStandardLabels(&deployment.ObjectMeta)
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
		setStandardLabels(&service.ObjectMeta)
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

func (r *WebSiteReconciler) reconcileNginxDeployment(ctx context.Context, webSite *websitev1beta1.WebSite, revision string) (bool, error) {
	log := r.log.WithValues("website", webSite.Name)
	deployment := &appsv1.Deployment{}
	deployment.SetNamespace(webSite.Namespace)
	deployment.SetName(webSite.Name)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, deployment, func() error {
		setStandardLabels(&deployment.ObjectMeta)
		deployment.Spec.Replicas = &webSite.Spec.Replicas
		deployment.Spec.Selector = &metav1.LabelSelector{}
		if deployment.Spec.Selector.MatchLabels == nil {
			deployment.Spec.Selector.MatchLabels = make(map[string]string)
		}
		deployment.Spec.Selector.MatchLabels[ManagedByKey] = OperatorName
		deployment.Spec.Selector.MatchLabels[AppNameKey] = AppName

		podTemplate, err := r.makeNginxPodTemplate(ctx, webSite, revision)
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

func (r *WebSiteReconciler) makeNginxPodTemplate(ctx context.Context, webSite *websitev1beta1.WebSite, revision string) (*corev1.PodTemplateSpec, error) {
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
	newTemplate.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup: pointer.Int64Ptr(10000),
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
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
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
		Command: []string{"/bin/bash", "-c", "/build/build.sh"},
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
		setStandardLabels(&service.ObjectMeta)
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

func setStandardLabels(om *metav1.ObjectMeta) {
	if om.Labels == nil {
		om.Labels = make(map[string]string)
	}
	om.Labels[ManagedByKey] = OperatorName
	om.Labels[AppNameKey] = AppName
}

func selectReadyWebSite(obj client.Object) []string {
	site := obj.(*websitev1beta1.WebSite)
	return []string{string(site.Status.Ready)}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebSiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &websitev1beta1.WebSite{}, website.WebSiteIndexField, selectReadyWebSite)
	if err != nil {
		return err
	}

	ch := make(chan event.GenericEvent)
	watcher := newRevisionWatcher(mgr.GetClient(), mgr.GetLogger().WithName("RevisionWatcher"), ch, 1*time.Minute, r.revisionClient)
	err = mgr.Add(watcher)
	if err != nil {
		return err
	}
	src := source.Channel{
		Source: ch,
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&websitev1beta1.WebSite{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&src, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
