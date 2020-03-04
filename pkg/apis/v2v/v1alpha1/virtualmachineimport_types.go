package v1alpha1

import (
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
	Source VirtualMachineImportSourceSpec `json:"source,omitempty"`
	Target VirtualMachineImportTargetSpec `json:"target,omitempty"`
}

// VirtualMachineImportTargetSpec defines the definition of the oVirt virtual machine infrastructure
// +k8s:openapi-gen=true
type VirtualMachineImportTargetSpec struct {
	Mapping []VirtualMachineImportOvirtTargetMappingSpec `json:"mapping"`
}

// VirtualMachineImportOvirtTargetMappingSpec defines the definition of the oVirt virtual machine infrastructure
// +k8s:openapi-gen=true
type VirtualMachineImportOvirtTargetMappingSpec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// VirtualMachineImportSourceSpec defines the definition of the oVirt virtual machine infrastructure
// +k8s:openapi-gen=true
type VirtualMachineImportSourceSpec struct {
	Ovirt VirtualMachineImportOvirtSourceSpec `json:"ovirt"`
}

// VirtualMachineImportOvirtSourceSpec defines the definition of the VM in oVirt
// +k8s:openapi-gen=true
type VirtualMachineImportOvirtSourceSpec struct {
	VM         VirtualMachineImportOvirtSourceVMSpec `json:"vm,omitempty"`
	SecretName string                                `json:"secretName,omitempty"`
}

// VirtualMachineImportOvirtSourceVMSpec defines the definition of the VM info in oVirt
// +k8s:openapi-gen=true
type VirtualMachineImportOvirtSourceVMSpec struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	ID      string `json:"id"`
}

// VirtualMachineImportStatus defines the observed state of VirtualMachineImport
// +k8s:openapi-gen=true
type VirtualMachineImportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	TargetVirtualMachineName string `json:"targetVirtualMachineName"`
	State                    string `json:"state"`  // FIXME: should be string?
	Phase                    string `json:"string"` // FIXME: should be string?
	Progress                 uint8  `json:"progress"`
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
