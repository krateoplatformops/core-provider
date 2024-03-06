package v1alpha1

import (
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true

type CompositionDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositionDefinition `json:"items"`
}

// +kubebuilder:validation:XValidation:rule="!has(oldSelf.version) || has(self.version)", message="Version is required once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.repo) || has(self.repo)", message="Repo is required once set"
type ChartInfo struct {
	// Url: oci or tgz full url
	// +immutable
	Url string `json:"url"`
	// Version: desired chart version
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Version is immutable"
	// +kubebuilder:validation:MaxLength=20
	Version string `json:"version,omitempty"`
	// Repo: helm repo name (for helm repo urls only)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Repo is immutable"
	// +kubebuilder:validation:MaxLength=256
	Repo string `json:"repo,omitempty"`
}

type CompositionDefinitionSpec struct {
	rtv1.ManagedSpec `json:",inline"`

	Chart *ChartInfo `json:"chart,omitempty"`
}

// CompositionDefinitionStatus is the status of a CompositionDefinition.
type CompositionDefinitionStatus struct {
	rtv1.ManagedStatus `json:",inline"`

	// Resource: the generated custom resource
	// +optional
	Resource string `json:"resource,omitempty"`

	// PackageURL: .tgz or oci chart direct url
	// +optional
	PackageURL string `json:"packageUrl,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,categories={krateo,definition,core}
//+kubebuilder:printcolumn:name="RESOURCE",type="string",JSONPath=".status.resource"
//+kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="PACKAGE URL",type="string",JSONPath=".status.packageUrl"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",priority=10

// CompositionDefinition is a definition type with a spec and a status.
type CompositionDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositionDefinitionSpec   `json:"spec,omitempty"`
	Status CompositionDefinitionStatus `json:"status,omitempty"`
}
