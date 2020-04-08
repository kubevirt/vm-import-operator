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
	Connect(*corev1.Secret) error
	Close()
	LoadVM(v2vv1alpha1.VirtualMachineImportSourceSpec) error
	PrepareResourceMapping(*v2vv1alpha1.ResourceMappingSpec, v2vv1alpha1.VirtualMachineImportSourceSpec)
	Validate() ([]v2vv1alpha1.VirtualMachineImportCondition, error)
	StopVM() error
	GetDataVolumeCredentials() DataVolumeCredentials
	UpdateVM(vmSpec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume)
	CreateMapper() Mapper
	GetVMStatus() (VMStatus, error)
	StartVM() error
	FindTemplate() (*oapiv1.Template, error)
	ProcessTemplate(*oapiv1.Template, string) (*kubevirtv1.VirtualMachine, error)
}

// Mapper is interface to be used for mapping external VM to kubevirt VM
type Mapper interface {
	CreateEmptyVM() *kubevirtv1.VirtualMachine
	MapVM(targetVMName *string, vmSpec *kubevirtv1.VirtualMachine) *kubevirtv1.VirtualMachine
	MapDisks() map[string]cdiv1.DataVolume
}

// DataVolumeCredentials defines the credentials required for creating a data volume
type DataVolumeCredentials struct {
	URL           string
	CACertificate string
	KeyAccess     string
	KeySecret     string
	ConfigMapName string
	SecretName    string
}

// VMStatus represents VM status
type VMStatus string
