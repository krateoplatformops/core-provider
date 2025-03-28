package v1alpha1

import (
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
type CompositionDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositionDefinition `json:"items"`
}

// GetItems of this CompositionDefinitionList.
func (l *CompositionDefinitionList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}

type Credentials struct {
	// Username: username for private repo
	Username string `json:"username"`
	// PasswordRef: reference to secret containing password for private repo
	PasswordRef rtv1.SecretKeySelector `json:"passwordRef"`
}

// +kubebuilder:validation:XValidation:rule="!has(oldSelf.version) || has(self.version)", message="Version is required once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.repo) || has(self.repo)", message="Repo is required once set"
type ChartInfo struct {
	// Url: oci or tgz full url
	Url string `json:"url"`
	// Version: desired chart version, needed for oci charts and for helm repo urls
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=20
	Version string `json:"version,omitempty"`
	// Repo: helm repo name (for helm repo urls only)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=256
	Repo string `json:"repo,omitempty"`

	// InsecureSkipVerifyTLS: skip tls verification
	// +optional
	InsecureSkipVerifyTLS bool `json:"insecureSkipVerifyTLS,omitempty"`

	// Credentials: credentials for private repos
	// +optional
	Credentials *Credentials `json:"credentials,omitempty"`
}

type ChartInfoProps struct {
	// Url: oci or tgz full url
	Url string `json:"url"`
	// Version: desired chart version, needed for oci charts and for helm repo urls
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=20
	Version string `json:"version,omitempty"`
	// Repo: helm repo name (for helm repo urls only)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=256
	Repo string `json:"repo,omitempty"`

	// InsecureSkipVerifyTLS: skip tls verification
	// +optional
	InsecureSkipVerifyTLS bool `json:"insecureSkipVerifyTLS,omitempty"`

	// Credentials: credentials for private repos
	// +optional
	Credentials *Credentials `json:"credentials,omitempty"`
}
type CompositionDefinitionSpec struct {
	// rtv1.ManagedSpec `json:",inline"`
	Chart *ChartInfo `json:"chart,omitempty"`
}

type VersionDetail struct {
	// Version: the version of the chart that is served. It is the version of the CRD.
	// +optional
	Version string `json:"version"`

	// Served: whether the version is served
	// +optional
	Served bool `json:"served"`

	// Stored: whether the version is stored
	// +optional
	Stored bool `json:"stored"`

	// Chart: the chart information
	// +optional
	Chart *ChartInfoProps `json:"chart"`
}

type Managed struct {
	// VersionInfo: the version information of the chart
	// +optional
	VersionInfo []VersionDetail `json:"versionInfo,omitempty"`

	// Group: the generated custom resource Group
	// +optional
	Group string `json:"group,omitempty"`

	// Kind: the generated custom resource Kind
	// +optional
	Kind string `json:"kind,omitempty"`
}

// CompositionDefinitionStatus is the status of a CompositionDefinition.
type CompositionDefinitionStatus struct {
	rtv1.ConditionedStatus `json:",inline"`

	// Kind: the kind of the custom resource - Last applied kind
	Kind string `json:"kind,omitempty"`

	// ApiVersion: the api version of the custom resource - Last applied apiVersion
	ApiVersion string `json:"apiVersion,omitempty"`

	// Managed: information about the managed resources
	Managed Managed `json:"managed,omitempty"`

	// PackageURL: .tgz or oci chart direct url
	// +optional
	PackageURL string `json:"packageUrl,omitempty"`

	// Digest: the digest of the managed resources
	// +optional
	Digest string `json:"digest,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,categories={krateo,defs,core}
//+kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="API VERSION",type="string",JSONPath=".status.apiVersion",priority=10
//+kubebuilder:printcolumn:name="KIND",type="string",JSONPath=".status.kind",priority=10
//+kubebuilder:printcolumn:name="PACKAGE URL",type="string",JSONPath=".status.packageUrl",priority=10

// CompositionDefinition is a definition type with a spec and a status.
type CompositionDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositionDefinitionSpec   `json:"spec,omitempty"`
	Status CompositionDefinitionStatus `json:"status,omitempty"`
}

// SetConditions of this CompositionDefinition.
func (mg *CompositionDefinition) SetConditions(c ...rtv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetCondition of this CompositionDefinition.
func (mg *CompositionDefinition) GetCondition(ct rtv1.ConditionType) rtv1.Condition {
	return mg.Status.GetCondition(ct)
}
