package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WebSiteSpec defines the desired state of WebSite
type WebSiteSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// BuildImage is a container image name that will be used to build the website
	// +kubebuiler:validation:Required
	BuildImage string `json:"buildImage"`

	// BuildScript is a script to build the website
	// +kubebuiler:validation:Required
	BuildScript DataSource `json:"buildScript"`

	// BuildSecrets is the list of secrets you can use in a build script
	// +optional
	BuildSecrets []SecretKey `json:"buildSecrets,omitempty"`

	// ImagePullSecrets is a list of references to secrets in the same namespace to use for pulling the images (buildImage, nginx and repo-checker).
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// RepoURL is the URL of the repository that has contents of the website
	// +kubebuiler:validation:Required
	RepoURL string `json:"repoURL"`

	// Branch is the branch name of the repository
	// +kubebuilder:default=main
	// +optional
	Branch string `json:"branch"`

	// DeployKeySecretName is the name of the secret resource that contains the deploy key to access the private repository
	// +optional
	DeployKeySecretName *string `json:"deployKeySecretName,omitempty"`

	// ExtraResources are resources that will be applied after the build step
	// +optional
	ExtraResources []DataSource `json:"extraResources,omitempty"`

	// Replicas is the number of nginx instances
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// PodTemplate is a `Pod` template for nginx container.
	// +optional
	PodTemplate *PodTemplate `json:"podTemplate,omitempty"`

	// ServiceTemplate is a `Service` template for nginx.
	// +optional
	ServiceTemplate *ServiceTemplate `json:"serviceTemplate,omitempty"`

	// JobScript is A script to execute in Job once after build
	// +optional
	JobScript DataSource `json:"jobScript"`
}

// SecretKey represents the name and key of a secret resource.
type SecretKey struct {
	// Name is the name of the secret resource
	Name string `json:"name"`
	// Key is the key of the secret resource
	Key string `json:"key"`
}

// DataSource represents the source of data.
// Only one of its members may be specified.
type DataSource struct {
	// ConfigMapName is the name of the ConfigMap
	// +optional
	ConfigMap *ConfigMapSource `json:"configMap,omitempty"`

	// RawData is raw data
	// +optional
	RawData *string `json:"rawData,omitempty"`
}

// PodTemplate defines the desired spec and annotations of Pod
type PodTemplate struct {
	// Standard object's metadata.  Only `annotations` and `labels` are valid.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`
}

// ServiceTemplate defines the desired spec and annotations of Service
type ServiceTemplate struct {
	// Standard object's metadata.  Only `annotations` and `labels` are valid.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`
}

// ObjectMeta is metadata of objects.
// This is partially copied from metav1.ObjectMeta.
type ObjectMeta struct {
	// Labels is a map of string keys and values.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is a map of string keys and values.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ConfigMapSource struct {
	// Name is the name of a configmap resource
	// +kubebuiler:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of a configmap resource
	// if omitted, it will be the same namespace as the WebSite resource
	// +optional
	Namespace string `json:"namespace"`

	// Key is the name of a key
	// +kubebuiler:validation:Required
	Key string `json:"key"`
}

// WebSiteStatus defines the observed state of WebSite
type WebSiteStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Revision is a revision currently available to the public
	Revision string `json:"revision"`
	// Ready is the current status
	Ready corev1.ConditionStatus `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="REVISION",type="string",JSONPath=".status.revision"

// WebSite is the Schema for the websites API
type WebSite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebSiteSpec   `json:"spec,omitempty"`
	Status WebSiteStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WebSiteList contains a list of WebSite
type WebSiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebSite `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebSite{}, &WebSiteList{})
}
