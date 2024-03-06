package v1alpha1

import (
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SchemaInfo struct {
	// Url of the values.schema.json file
	// +kubebuilder:validation:Required
	Url string `json:"url"`

	// Version: allow Kubernetes to release groups as tagged versions.
	// +kubebuilder:validation:Optional
	Version *string `json:"version,omitempty"`

	// Kind: the name of the object you are trying to generate
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// SchemaDefinitionSpec is the specification of a Definition.
type SchemaDefinitionSpec struct {
	rtv1.ManagedSpec `json:",inline"`

	// Schema: the schema info
	// +immutable
	Schema SchemaInfo `json:"schema"`
}

// SchemaDefinitionStatus is the status of a Definition.
type SchemaDefinitionStatus struct {
	rtv1.ManagedStatus `json:",inline"`

	// Resource: the generated custom resource
	// +optional
	Resource string `json:"resource,omitempty"`

	// Digest: schema digest
	// +optional
	Digest *string `json:"digest,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,categories={krateo,definition,core}
//+kubebuilder:printcolumn:name="RESOURCE",type="string",JSONPath=".status.resource"
//+kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",priority=10

// SchemaDefinition is a definition type with a spec and a status.
type SchemaDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchemaDefinitionSpec   `json:"spec,omitempty"`
	Status SchemaDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SchemaDefinitionList is a list of Definition objects.
type SchemaDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SchemaDefinition `json:"items"`
}
