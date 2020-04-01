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
	ProviderCredentialsSecret ObjectIdentifier               `json:"providerCredentialsSecret"`
	ResourceMapping           *ObjectIdentifier              `json:"resourceMapping,omitempty"`
	Source                    VirtualMachineImportSourceSpec `json:"source"`

	// +optional
	TargetVMName *string `json:"targetVmName,omitempty"`

	// +optional
	StartVM *bool `json:"startVm,omitempty"`
}

// VirtualMachineImportSourceSpec defines the definition of the source provider and mapping resources
// +k8s:openapi-gen=true
// +optional
type VirtualMachineImportSourceSpec struct {
	Ovirt *VirtualMachineImportOvirtSourceSpec `json:"ovirt,omitempty"`
}

// VirtualMachineImportOvirtSourceSpec defines the definition of the VM in oVirt and the credentials to oVirt
// +k8s:openapi-gen=true
type VirtualMachineImportOvirtSourceSpec struct {
	VM VirtualMachineImportOvirtSourceVMSpec `json:"vm"`

	// +optional
	Mappings *OvirtMappings `json:"mappings,omitempty"`
}

// ObjectIdentifier defines how a resource should be identified on kubevirt
// +k8s:openapi-gen=true
type ObjectIdentifier struct {
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
	// +optional
	ID *string `json:"id,omitempty"`

	// +optional
	Name *string `json:"name,omitempty"`
}

// VirtualMachineImportStatus defines the observed state of VirtualMachineImport
// +k8s:openapi-gen=true
type VirtualMachineImportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	TargetVMName string                          `json:"targetVmName"`
	Conditions   []VirtualMachineImportCondition `json:"conditions"`
	DataVolumes  []DataVolumeItem                `json:"dataVolumes"`
}

// VirtualMachineImportConditionType defines the condition of VM import
// +k8s:openapi-gen=true
type VirtualMachineImportConditionType string

// These are valid conditions of of VM import.
const (
	// Succeeded represents status of the VM import process being completed successfully
	Succeeded VirtualMachineImportConditionType = "Succeeded"

	// Validating represents the status of the validation of the mapping rules and eligibility of source VM for import
	Validating VirtualMachineImportConditionType = "Validating"

	// MappingRulesChecking represents the status of the VM import mapping rules checking
	MappingRulesChecking VirtualMachineImportConditionType = "MappingRulesChecking"

	// Processing represents the status of the VM import process while in progress
	Processing VirtualMachineImportConditionType = "Processing"
)

// SucceededConditionReason defines the reasons for the Succeeded condition of VM import
// +k8s:openapi-gen=true
type SucceededConditionReason string

// These are valid reasons for the Succeeded conditions of VM import.
const (
	// ValidationFailed represents a failure to validate the eligibility of the VM for import
	ValidationFailed SucceededConditionReason = "ValidationFailed"

	// UpdatingSourceVMFailed represents a failure to stop source VM
	UpdatingSourceVMFailed SucceededConditionReason = "UpdatingSourceVMFailed"

	// VMCreationFailed represents a failure to create the VM entity
	VMCreationFailed SucceededConditionReason = "VMCreationFailed"

	// DataVolumeCreationFailed represents a failure to create data volumes based on source VM disks
	DataVolumeCreationFailed SucceededConditionReason = "DataVolumeCreationFailed"

	// VirtualMachineReady represents the completion of the vm import
	VirtualMachineReady SucceededConditionReason = "VirtualMachineReady"

	// VirtualMachineRunning represents the completion of the vm import and vm in running state
	VirtualMachineRunning SucceededConditionReason = "VirtualMachineRunning"
)

// ValidatingConditionReason defines the reasons for the Validating condition of VM import
// +k8s:openapi-gen=true
type ValidatingConditionReason string

// These are valid reasons for the Validating conditions of VM import.
const (
	// ValidationCompleted represents the completion of the vm import resource validating
	ValidationCompleted ValidatingConditionReason = "ValidationCompleted"

	// SecretNotFound represents the nonexistence of the provider's secret
	SecretNotFound ValidatingConditionReason = "SecretNotFound"

	// MappingResourceNotFound represents the nonexistence of the mapping resource
	MappingResourceNotFound ValidatingConditionReason = "MappingResourceNotFound"

	// UnreachableProvider represents a failure to connect to the provider
	UnreachableProvider ValidatingConditionReason = "UnreachableProvider"

	// SourceVmNotFound represents the nonexistence of the source VM
	SourceVMNotFound ValidatingConditionReason = "SourceVMNotFound"

	// IncompleteMappingRules represents the inability to prepare the mapping rules
	IncompleteMappingRules ValidatingConditionReason = "IncompleteMappingRules"
)

// MappingRulesCheckingReason defines the reasons for the MappingRulesChecking condition of VM import
// +k8s:openapi-gen=true
type MappingRulesCheckingReason string

// These are valid reasons for the Validating conditions of VM import.
const (
	// Completed represents the completion of the mapping rules checking without warnings or errors
	MappingRulesCheckingCompleted MappingRulesCheckingReason = "MappingRulesCheckingCompleted"

	// MappingRulesViolated represents the violation of the mapping rules
	MappingRulesCheckingFailed MappingRulesCheckingReason = "MappingRulesCheckingFailed"

	// MappingRulesWarningsReported represents the existence of warnings as a result of checking the mapping rules
	MappingRulesCheckingReportedWarnings MappingRulesCheckingReason = "MappingRulesCheckingReportedWarnings"
)

// ProcessingConditionReason defines the reasons for the Processing condition of VM import
// +k8s:openapi-gen=true
type ProcessingConditionReason string

// These are valid reasons for the Processing conditions of VM import.
const (
	// UpdatingSourceVM represents the renaming of source vm to be prefixed with 'imported_' and shutting it down
	UpdatingSourceVM ProcessingConditionReason = "UpdatingSourceVM"

	// CreatingTargetVM represents the creation of the VM spec
	CreatingTargetVM ProcessingConditionReason = "CreatingTargetVM"

	// CopyingDisks represents the creation of data volumes based on source VM disks
	CopyingDisks ProcessingConditionReason = "CopyingDisks"
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
