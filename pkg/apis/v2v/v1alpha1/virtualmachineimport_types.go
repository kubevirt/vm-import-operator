package v1alpha1

import (
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VirtualMachineImportSpec defines the desired state of VirtualMachineImport
// +k8s:openapi-gen=true
type VirtualMachineImportSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Source           VirtualMachineImportSourceSpec `json:"source"`
	ResourceMappings ResourceMappingsSpec           `json:"target"`

	// +optional
	TargetVirtualMachineName *string `json:"targetVirtualMachineName,omitempty"`
}

// ResourceMappingsSpec defines the definition of the config map that holds the resources mapping between source provider to kubevirt
// +k8s:openapi-gen=true
type ResourceMappingsSpec struct {
	ConfigMapName string `json:"configMapName"`

	// +optional
	ConfigMapNamespace *string `json:"configMapNamespace,omitempty"`
}

// VirtualMachineImportSourceSpec defines the definition of the oVirt virtual machine infrastructure
// +k8s:openapi-gen=true
// +optional
type VirtualMachineImportSourceSpec struct {
	Ovirt *VirtualMachineImportOvirtSourceSpec `json:"ovirt,omitempty"`
}

// VirtualMachineImportOvirtSourceSpec defines the definition of the VM in oVirt and the credentials to oVirt
// +k8s:openapi-gen=true
type VirtualMachineImportOvirtSourceSpec struct {
	VM                        VirtualMachineImportOvirtSourceVMSpec `json:"vm"`
	ProviderCredentialsSecret ProviderCredentialsSecret             `json:"providerCredentialsSecret"`
}

// ProviderCredentialsSecret defines the details of the secret that contains the credentials to oVirt
// +k8s:openapi-gen=true
type ProviderCredentialsSecret struct {
	Name string `json:"name"`

	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// VirtualMachineImportOvirtSourceVMSpec defines the definition of the VM info in oVirt
// +k8s:openapi-gen=true
type VirtualMachineImportOvirtSourceVMSpec struct {
	// +optional
	ID *string `json:"id,omitempty"`

	// +optional
	Name *string `json:"name,omitempty"`

	// +optional
	Cluster *VirtualMachineImportOvirtSourceVMClusterSpec `json:"cluster,omitempty"`
}

// VirtualMachineImportOvirtSourceVMClusterSpec defines the definition of the source cluster of the VM in oVirt
// +k8s:openapi-gen=true
// +optional
type VirtualMachineImportOvirtSourceVMClusterSpec struct {
	ID string `json:"id"`

	// +optional
	Name *string `json:"name,omitempty"`
}

// VirtualMachineImportStatus defines the observed state of VirtualMachineImport
// +k8s:openapi-gen=true
type VirtualMachineImportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	TargetVirtualMachineName string                          `json:"targetVirtualMachineName"`
	Conditions               []VirtualMachineImportCondition `json:"conditions"`
	State                    VirtualMachineImportState       `json:"state"`
	DataVolumes              []DataVolumeItem                `json:"dataVolumes"`
}

// VirtualMachineImportState defines the state of virtual machine import
// +k8s:openapi-gen=true
type VirtualMachineImportState string

// These are valid values of virtual machine import state
const (
	VirtualMachineImportStateRunning  VirtualMachineImportState = "Running"
	VirtualMachineImportStateFinished VirtualMachineImportState = "Finished"
	VirtualMachineImportStateUnknown  VirtualMachineImportState = "Unknown"
)

// VirtualMachineImportConditionType defines the condition of VM import
// +k8s:openapi-gen=true
type VirtualMachineImportConditionType string

// These are valid conditions of of VM import.
const (
	// Ready represents status of the VM import process being completed.
	Ready VirtualMachineImportConditionType = "Ready"

	// Processing represents status of the VM import process while in progress
	Processing VirtualMachineImportConditionType = "Processing"
)

// VirtualMachineImportCondition defines the observed state of VirtualMachineImport conditions
// +k8s:openapi-gen=true
type VirtualMachineImportCondition struct {
	// Type of virtual machine import condition
	Type VirtualMachineImportConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown
	Status k8sv1.ConditionStatus `json:"status"`

	// A brief CamelCase string that describes why the VM import process is in current condition status
	// +optional
	Reason *string `json:"reason,omitempty"`

	// A human-readable message indicating details about last transition
	// +optional
	Message *string `json:"message,omitempty"`

	// The last time we got an update on a given condition
	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// The last time the condition transit from one status to another
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// DataVolumeItem defines the details of a data volume created by the VM import process
// +k8s:openapi-gen=true
type DataVolumeItem struct {
	Name string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachineImport is the Schema for the virtualmachineimports API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type VirtualMachineImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineImportSpec   `json:"spec,omitempty"`
	Status VirtualMachineImportStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachineImportList contains a list of VirtualMachineImport
type VirtualMachineImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineImport{}, &VirtualMachineImportList{})
}
