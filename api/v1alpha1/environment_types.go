package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentKind is the kind of the Environment resource.
const EnvironmentKind = "Environment"

// Condition types reported on an Environment.
const (
	// ConditionReady is true when Crossplane, packages and tools are installed
	// and ready.
	ConditionReady = "Ready"
)

// ManagementPolicy defines how the operator treats a component that may
// already exist in the cluster.
// +kubebuilder:validation:Enum=Auto;Adopt;External
type ManagementPolicy string

const (
	// ManagementAuto installs the component when it is missing and uses an
	// existing installation as-is, without taking ownership of it.
	ManagementAuto ManagementPolicy = "Auto"
	// ManagementAdopt installs the component when it is missing and takes
	// ownership of an existing Helm release with the expected name, upgrading
	// it to the desired version and values.
	ManagementAdopt ManagementPolicy = "Adopt"
	// ManagementExternal never installs or modifies the component; it must be
	// provided by someone else.
	ManagementExternal ManagementPolicy = "External"
)

// DeletionPolicy defines what happens to owned components when the
// Environment is deleted.
// +kubebuilder:validation:Enum=Orphan;Delete
type DeletionPolicy string

const (
	// DeletionOrphan keeps all components in the cluster.
	DeletionOrphan DeletionPolicy = "Orphan"
	// DeletionDelete removes the components owned by the Environment.
	// Components not owned by it are never removed.
	DeletionDelete DeletionPolicy = "Delete"
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

	// Registries configure how Crossplane resolves package images: prefix
	// rewrites (e.g. to a local registry) and registry authentication. Each
	// entry becomes a Crossplane ImageConfig, reconciled before packages.
	// Rewrites also apply to package dependencies.
	// +listType=map
	// +listMapKey=name
	// +optional
	Registries []RegistrySpec `json:"registries,omitempty"`

	// ImageStore serves the images of the nodes' containerd to Crossplane as a
	// registry, so packages are installed without a separate registry. Images
	// of upstream registries are cached in containerd on first use.
	// +optional
	ImageStore *ImageStoreSpec `json:"imageStore,omitempty"`

	// Scheduling places Crossplane, tools and package runtimes on matching
	// nodes, e.g. on dedicated engine nodes.
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`

	// DeletionPolicy defines what happens to owned components when the
	// Environment is deleted. Defaults to Orphan.
	// +kubebuilder:default=Orphan
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// RegistrySpec is a rule for package images whose reference starts with one
// of the prefixes.
type RegistrySpec struct {
	// Name of the rule, unique in the Environment.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=50
	Name string `json:"name"`

	// Prefixes of fully qualified package images the rule applies to, e.g.
	// "xpkg.crossplane.io/crossplane-contrib/". The longest matching prefix of
	// all rules wins.
	// +kubebuilder:validation:MinItems=1
	Prefixes []string `json:"prefixes"`

	// RewritePrefix replaces the matched prefix, e.g.
	// "registry.overlock.svc.cluster.local/crossplane-contrib/".
	// +optional
	RewritePrefix string `json:"rewritePrefix,omitempty"`

	// PullSecret is the name of a Secret of type kubernetes.io/dockerconfigjson
	// in the Crossplane namespace used to pull matching images.
	// +optional
	PullSecret string `json:"pullSecret,omitempty"`
}

// ImageStoreSpec configures the image store.
type ImageStoreSpec struct {
	// Enabled deploys the image store on every node and makes Crossplane
	// trust it. Requires cert-manager.
	Enabled bool `json:"enabled"`

	// ContainerdSocket is the path of the containerd socket on the nodes.
	// Defaults to the K3s socket.
	// +optional
	ContainerdSocket string `json:"containerdSocket,omitempty"`
}

// ImageStoreEnabled reports whether the image store is enabled.
func (s *EnvironmentSpec) ImageStoreEnabled() bool {
	return s.ImageStore != nil && s.ImageStore.Enabled
}

// SchedulingSpec constrains the nodes owned components run on.
type SchedulingSpec struct {
	// NodeSelector applied to Crossplane, tools and package runtimes.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations applied to Crossplane, tools and package runtimes.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// CrossplaneSpec configures the Crossplane engine.
type CrossplaneSpec struct {
	// Version of the Crossplane Helm chart, at least 2.0.0. Defaults to the
	// version shipped with the operator.
	// +optional
	Version string `json:"version,omitempty"`

	// Policy defines how an existing Crossplane installation is treated.
	// Defaults to Auto.
	// +optional
	Policy ManagementPolicy `json:"policy,omitempty"`

	// MaxReconcileRate is the maximum number of reconciliations per second for Crossplane.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxReconcileRate int `json:"maxReconcileRate,omitempty"`

	// Args are additional arguments passed to the Crossplane core container.
	// +optional
	Args []string `json:"args,omitempty"`

	// RegistryCABundle is a ConfigMap key in the Crossplane namespace with
	// additional CA certificates Crossplane trusts when pulling packages,
	// e.g. of a local registry.
	// +optional
	RegistryCABundle *ConfigMapKey `json:"registryCABundle,omitempty"`
}

// ConfigMapKey selects a key of a ConfigMap.
type ConfigMapKey struct {
	// Name of the ConfigMap.
	Name string `json:"name"`
	// Key in the ConfigMap.
	Key string `json:"key"`
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
	// Enabled installs the tool. Defaults to true. Disabling a tool owned by
	// the Environment removes it; tools not owned by it are left untouched.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Version of the tool Helm chart. Defaults to the version shipped with
	// the operator.
	// +optional
	Version string `json:"version,omitempty"`

	// Policy defines how an existing installation of the tool is treated.
	// Defaults to Auto.
	// +optional
	Policy ManagementPolicy `json:"policy,omitempty"`
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

// ComponentState is the observed state of a component.
type ComponentState string

const (
	// ComponentInstalled is installed and owned by the Environment.
	ComponentInstalled ComponentState = "Installed"
	// ComponentExternal is installed and managed outside of the Environment.
	ComponentExternal ComponentState = "External"
	// ComponentMissing is required but not installed (External policy).
	ComponentMissing ComponentState = "Missing"
	// ComponentDisabled is disabled in the specification.
	ComponentDisabled ComponentState = "Disabled"
	// ComponentPending waits for a dependency.
	ComponentPending ComponentState = "Pending"
	// ComponentFailed failed to install, upgrade or remove.
	ComponentFailed ComponentState = "Failed"
)

// ComponentStatus is the observed state of a single component.
type ComponentStatus struct {
	// Name of the component, e.g. crossplane, cert-manager or
	// provider/provider-helm.
	Name string `json:"name"`

	// State of the component.
	State ComponentState `json:"state"`

	// Owned is true when the Environment owns, and may upgrade or remove, the
	// component.
	// +optional
	Owned bool `json:"owned,omitempty"`

	// Ready is true when the component is available.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Version of the component, when known.
	// +optional
	Version string `json:"version,omitempty"`

	// Message with details about the state.
	// +optional
	Message string `json:"message,omitempty"`
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

	// Components describe the state of each installed component.
	// +listType=map
	// +listMapKey=name
	// +optional
	Components []ComponentStatus `json:"components,omitempty"`
}

// Environment is the Schema for the environments API. It is cluster-scoped:
// one Environment describes what the operator installs in its own cluster.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=env
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
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
