package provider

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

// Provider defines the methods required by source providers for importing a VM
type Provider interface {
	Connect(*corev1.Secret) error
	Close()
	LoadVM(v2vv1alpha1.VirtualMachineImportSourceSpec) error
	PrepareResourceMapping(*v2vv1alpha1.ResourceMappingSpec, v2vv1alpha1.VirtualMachineImportSourceSpec)
	Validate() error
	StopVM() error
	GetDataVolumeCredentials() DataVolumeCredentials
	UpdateVM(vmSpec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume)
	CreateMapper() Mapper
}

// Mapper is interface to be used for mapping external VM to kubevirt VM
type Mapper interface {
	MapVM(targetVMName *string) *kubevirtv1.VirtualMachine
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
