/*
Copyright 2021 The Cockroach Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// CrdbClusterSpec defines the desired state of a CockroachDB Cluster
// that the operator maintains.
// +k8s:openapi-gen=true
type CrdbClusterSpec struct {
	// Number of nodes (pods) in the cluster
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Number of nodes",xDescriptors="urn:alm:descriptor:com.tectonic.ui:podCount"
	// +required
	Nodes int32 `json:"nodes"`
	// (Required) Container image information
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cockroach Database Image"
	// +optional
	Image PodImage `json:"image"`
	// (Optional) The database port (`--port` CLI parameter when starting the service)
	// Default: 26257
	// +optional
	GRPCPort *int32 `json:"grpcPort,omitempty"`
	// The web UI port (`--http-port` CLI parameter when starting the service)
	// Default: 8080
	// +optional
	HTTPPort *int32 `json:"httpPort,omitempty"`
	// (Optional) TLSEnabled determines if TLS is enabled for your CockroachDB Cluster
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLS Enabled",xDescriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +optional
	TLSEnabled bool `json:"tlsEnabled,omitempty"`
	// (Optional) The secret with certificates and a private key for the TLS endpoint
	// on the database port. The standard naming of files is expected (tls.key, tls.crt, ca.crt)
	// Default: ""
	// +optional
	NodeTLSSecret string `json:"nodeTLSSecret,omitempty"`
	// (Optional) The secret with a certificate and a private key for root database user
	// Default: ""
	// +optional
	ClientTLSSecret string `json:"clientTLSSecret,omitempty"`
	// (Optional) The maximum number of pods that can be unavailable during a rolling update.
	// This number is set in the PodDistruptionBudget and defaults to 1.
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
	// (Optional) The min number of pods that can be unavailable during a rolling update.
	// This number is set in the PodDistruptionBudget and defaults to 1.
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`
	// (Optional) The total size for caches (`--cache` command line parameter)
	// Default: "25%"
	// +optional
	Cache string `json:"cache,omitempty"`
	// (Optional) The maximum in-memory storage capacity available to store temporary
	// data for SQL queries (`--max-sql-memory` parameter)
	// Default: "25%"
	// +optional
	MaxSQLMemory string `json:"maxSQLMemory,omitempty"`
	// (Optional) Additional command line arguments for the `cockroach` binary
	// Default: ""
	// +optional
	AdditionalArgs []string `json:"additionalArgs,omitempty"`
	// (Optional) Database container resource limits. Any container limits
	// can be specified.
	// Default: (not specified)
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// (Required) Database disk storage configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Data Store"
	// +required
	DataStore Volume `json:"dataStore,omitempty"`
	// (Optional) Additional custom resource labels
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Map of additional custom labels"
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// CrdbClusterStatus defines the observed state of Cluster
type CrdbClusterStatus struct {
	// List of conditions representing the current status of the cluster resource.
	// +operator-sdk:csv:customresourcedefinitions:type=status, displayName="Cluster Conditions",xDescriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=status, displayName="Operator Conditions",xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	OperatorConditions []ClusterCondition `json:"operatorConditions,omitempty"`
	// Database service version. Not populated and is just a placeholder currently.
	// +operator-sdk:csv:customresourcedefinitions:type=status, displayName="Version",xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	Version string `json:"version,omitempty"`
}

// ClusterCondition represents cluster status as it is perceived by
// the operator
type ClusterCondition struct {
	// +required
	// Type/Name of the condition
	Type ClusterConditionType `json:"type"`
	// +required
	// Condition status: True, False or Unknown
	Status metav1.ConditionStatus `json:"status"`
	// +required
	// The time when the condition was updated
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;cockroachdb,shortName=crdb
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="CockroachDB Operator"
// CrdbCluster is the CRD for the cockroachDB clusters API
type CrdbCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CrdbClusterSpec   `json:"spec,omitempty"`
	Status CrdbClusterStatus `json:"status,omitempty"`
}

// PodImage represents the image information for a container that is used
// to build the StatefulSet.
// +kubebuilder:object:generate=true
// +k8s:openapi-gen=true
type PodImage struct {
	// Container image with supported CockroachDB version.
	// This defaults to the version pinned to the operator and requires a full container and tag/sha name.
	// For instance: cockroachdb/cockroachdb:v20.1
	// +required
	Name string `json:"name,omitempty"`
	// PullPolicy for the image, which defaults to IfNotPresent.
	// +required
	PullPolicyName *v1.PullPolicy `json:"pullPolicy,omitempty"`
	// Secret name containing the dockerconfig to use for a
	// registry that requires authentication. The secret
	// must be configured first by the user.
	// +optional
	PullSecret *string `json:"pullSecret,omitempty"`
}

// Volume defined storage configuration for the container with the Database.
// Only one of the fields should set
// +k8s:openapi-gen=true
// +kubebuilder:object:generate=true
type Volume struct {
	// Directory from the host node's filesystem
	// +optional
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`
	// Temporary folder on the host node's filesystem
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	// Persistent volume to use
	// +optional
	VolumeClaim *VolumeClaim `json:"pvc,omitempty"`
	// SupportsAutoResize marks that a PVC will resize without restarting the entire cluster
	// Default: false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="PVC Supports Auto Resizing",xDescriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	// +optional
	SupportsAutoResize bool `json:"supportsAutoResize"`
}

// VolumeClaim wraps a persistent volume claim (PVC) to use with the container.
// Only one of the fields should set
// +kubebuilder:object:generate=true
// +k8s:openapi-gen=true
type VolumeClaim struct {
	// PVC to request a new persistent volume
	// +required
	PersistentVolumeClaimSpec corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
	// Existing PVC in the same namespace
	// +required
	PersistentVolumeSource corev1.PersistentVolumeClaimVolumeSource `json:"source,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=all;addons

// CrdbClusterList contains a list of Cluster
type CrdbClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CrdbCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CrdbCluster{}, &CrdbClusterList{})
}
