package v1alpha1

import (
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true

// DefinitionList is a list of Definition objects.
type DefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Definition `json:"items"`
}

type ChartInfo struct {
	Url     string `json:"url"`
	Version string `json:"version"`
	Name    string `json:"name"`
}

// DefinitionSpec is the specification of a Definition.
type DefinitionSpec struct {
	rtv1.ManagedSpec `json:",inline"`

	Chart *ChartInfo `json:"chart,omitempty"`
}

// DefinitionStatus is the status of a Definition.
type DefinitionStatus struct {
	rtv1.ManagedStatus `json:",inline"`

	// Resource: the generated custom resource
	// +optional
	Resource string `json:"resource,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,categories={krateo,definition,core}
//+kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.chart.version"
//+kubebuilder:printcolumn:name="RESOURCE",type="string",JSONPath=".status.resource"
//+kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",priority=10
//+kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.chart.url",priority=10

// Definition is a definition type with a spec and a status.
type Definition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DefinitionSpec   `json:"spec,omitempty"`
	Status DefinitionStatus `json:"status,omitempty"`
}
