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

	// PreBuildResources are resources that will be applied before the build step
	// +optional
	PreBuildResources []DataSource `json:"preBuildResources,omitempty"`

	// BuildImage is the container image name that will be used to build the website
	// +kubebuiler:validation:Required
	BuildImage string `json:"buildImage"`

	// BuildScript is the script to build the website
	// +kubebuiler:validation:Required
	BuildScript DataSource `json:"buildScript"`

	// RepoURL is the URL of the repository that has contents of the website
	// +kubebuiler:validation:Required
	RepoURL string `json:"repoURL"`

	// RepoURL is the branch name of the repository
	// +kubebuilder:default=main
	// +optional
	Branch string `json:"branch"`

	// DeployKeySecretName is the name of the secret resource that contains the deploy key to access the private repository
	// +optional
	DeployKeySecretName *string `json:"deployKeySecretName,omitempty"`

	// PostBuildResources are resources that will be applied after the build step
	// +optional
	PostBuildResources []DataSource `json:"postBuildResources,omitempty"`
}

// DataSource represents the source of data.
// Only one of its members may be specified.
type DataSource struct {
	// ConfigMapName is the name of the ConfigMap
	// +optional
	ConfigMapName *string `json:"configMapName,omitempty"`

	// RawData is raw data
	// +optional
	RawData *string `json:"rawData,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="REVISION",type="string",JSONPath=".status.revision"

// WebSite is the Schema for the websites API
type WebSite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebSiteSpec   `json:"spec,omitempty"`
	Status WebSiteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WebSiteList contains a list of WebSite
type WebSiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebSite `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebSite{}, &WebSiteList{})
}
