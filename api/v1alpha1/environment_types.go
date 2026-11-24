package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentKind is the kind of the Environment resource.
const EnvironmentKind = "Environment"

// Condition types reported on an Environment.
const (
	// ConditionReady is true when Crossplane, packages and tools are installed.
	ConditionReady = "Ready"
)

// EnvironmentSpec is the operator-compatible specification of an Overlock
// environment: what is installed inside a Kubernetes cluster. How the cluster
// itself is created (engine, nodes, SSH, Docker, WireGuard) is CLI-specific
// and intentionally not part of this API.
type EnvironmentSpec struct {
	// Crossplane configures the Crossplane engine installation.
	// +optional
	Crossplane CrossplaneSpec `json:"crossplane,omitempty"`

	// Providers are Crossplane provider packages to install.
	// +optional
	Providers []string `json:"providers,omitempty"`

	// Configurations are Crossplane configuration packages to install.
	// +optional
	Configurations []string `json:"configurations,omitempty"`

	// Functions are Crossplane function packages to install.
	// +optional
	Functions []string `json:"functions,omitempty"`

	// Tools configures supporting tools installed next to Crossplane.
	// +optional
	Tools ToolsSpec `json:"tools,omitempty"`

	// AdminServiceAccount creates a service account with cluster-admin privileges.
	// +optional
	AdminServiceAccount *AdminServiceAccountSpec `json:"adminServiceAccount,omitempty"`
}

// CrossplaneSpec configures the Crossplane engine.
type CrossplaneSpec struct {
	// MaxReconcileRate is the maximum number of reconciliations per second for Crossplane.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxReconcileRate int `json:"maxReconcileRate,omitempty"`

	// Args are additional arguments passed to the Crossplane core container.
	// +optional
	Args []string `json:"args,omitempty"`
}

// ToolsSpec configures supporting tools. Every tool is enabled unless
// explicitly disabled.
type ToolsSpec struct {
	// Kyverno policy engine.
	// +optional
	Kyverno ToolSpec `json:"kyverno,omitempty"`

	// CertManager certificate manager.
	// +optional
	CertManager ToolSpec `json:"certManager,omitempty"`
}

// ToolSpec configures a single tool.
type ToolSpec struct {
	// Enabled installs the tool. Defaults to true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// IsEnabled reports whether the tool should be installed (default true).
func (t ToolSpec) IsEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// AdminServiceAccountSpec configures the admin service account.
type AdminServiceAccountSpec struct {
	// Name of the service account. Defaults to "overlock-admin".
	// +optional
	Name string `json:"name,omitempty"`
}

// EnvironmentStatus is the observed state of an Environment.
type EnvironmentStatus struct {
	// ObservedGeneration is the generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions describe the current state of the environment.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Environment is the Schema for the environments API. It is cluster-scoped:
// one Environment describes what the operator installs in its own cluster.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=env
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// EnvironmentList contains a list of Environment.
//
// +kubebuilder:object:root=true
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
