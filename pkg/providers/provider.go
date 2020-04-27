package provider

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	oapiv1 "github.com/openshift/api/template/v1"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	// VMStatusDown defines VM status representing stopped VM
	VMStatusDown VMStatus = "down"
	// VMStatusUp defines VM status representing running VM
	VMStatusUp VMStatus = "up"
)

// Provider defines the methods required by source providers for importing a VM
type Provider interface {
	Init(*corev1.Secret, *v2vv1alpha1.VirtualMachineImport) error
	Close()
	LoadVM(v2vv1alpha1.VirtualMachineImportSourceSpec) error
	PrepareResourceMapping(*v2vv1alpha1.ResourceMappingSpec, v2vv1alpha1.VirtualMachineImportSourceSpec)
	Validate() ([]v2vv1alpha1.VirtualMachineImportCondition, error)
	StopVM() error
	CreateMapper() (Mapper, error)
	GetVMStatus() (VMStatus, error)
	GetVMName() (string, error)
	StartVM() error
	CleanUp(bool) error
	FindTemplate() (*oapiv1.Template, error)
	ProcessTemplate(*oapiv1.Template, *string, string) (*kubevirtv1.VirtualMachine, error)
}

// Mapper is interface to be used for mapping external VM to kubevirt VM
type Mapper interface {
	CreateEmptyVM(vmName *string) *kubevirtv1.VirtualMachine
	ResolveVMName(targetVMName *string) *string
	MapVM(targetVMName *string, vmSpec *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error)
	MapDataVolumes() (map[string]cdiv1.DataVolume, error)
	MapDisks(vmSpec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume)
}

// VMStatus represents VM status
type VMStatus string
