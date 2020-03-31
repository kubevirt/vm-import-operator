package validators

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"kubevirt.io/client-go/kubecli"
)

// ValidatorWrapper exposes validator package API as a struct
type ValidatorWrapper struct {
	networkMappingValidator NetworkMappingValidator
	storageMappingValidator StorageMappingValidator
}

// NewValidatorWrapper creates new, configured ValidatorWrapper
func NewValidatorWrapper(kubevirt kubecli.KubevirtClient) *ValidatorWrapper {
	netAttachDefProvider := NetworkAttachmentDefinitions{
		Client: kubevirt.NetworkClient(),
	}
	storageClassesProvider := StorageClasses{
		Client: kubevirt.StorageV1().StorageClasses(),
	}
	return &ValidatorWrapper{
		networkMappingValidator: NewNetworkMappingValidator(&netAttachDefProvider),
		storageMappingValidator: NewStorageMappingValidator(&storageClassesProvider),
	}
}

// ValidateVM wraps validators package implementation of ValidateVM function
func (v *ValidatorWrapper) ValidateVM(vm *ovirtsdk.Vm) []ValidationFailure {
	return ValidateVM(vm)
}

// ValidateDiskAttachments wraps validators package implementation of ValidateDiskAttachments function
func (v *ValidatorWrapper) ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []ValidationFailure {
	return ValidateDiskAttachments(diskAttachments)
}

// ValidateNics wraps validators package implementation of ValidateNics function
func (v *ValidatorWrapper) ValidateNics(nics []*ovirtsdk.Nic) []ValidationFailure {
	return ValidateNics(nics)
}

// ValidateNetworkMapping wraps networkMappingValidator call
func (v *ValidatorWrapper) ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1alpha1.ResourceMappingItem, crNamespace string) []ValidationFailure {
	return v.networkMappingValidator.ValidateNetworkMapping(nics, mapping, crNamespace)
}

// ValidateStorageMapping wraps storageMappingValidator call
func (v *ValidatorWrapper) ValidateStorageMapping(attachments []*ovirtsdk.DiskAttachment, mapping *[]v2vv1alpha1.ResourceMappingItem) []ValidationFailure {
	return v.storageMappingValidator.ValidateStorageMapping(attachments, mapping)
}
