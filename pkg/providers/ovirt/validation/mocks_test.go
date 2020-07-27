package validation_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	validators "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

var validateVMMock func(*ovirtsdk.Vm) []validators.ValidationFailure
var validateNicsMock func([]*ovirtsdk.Nic) []validators.ValidationFailure
var validateDiskAttachmentsMock func([]*ovirtsdk.DiskAttachment) []validators.ValidationFailure
var validateNetworkMappingsMock func(nics []*ovirtsdk.Nic, mapping *[]v2vv1alpha1.NetworkResourceMappingItem, crNamespace string) []validators.ValidationFailure
var validateStorageMappingMock func(
	attachments []*ovirtsdk.DiskAttachment,
	storageMapping *[]v2vv1alpha1.StorageResourceMappingItem,
	diskMapping *[]v2vv1alpha1.StorageResourceMappingItem,
) []validators.ValidationFailure

type mockValidator struct{}

func (v *mockValidator) ValidateVM(vm *ovirtsdk.Vm) []validators.ValidationFailure {
	return validateVMMock(vm)
}

func (v *mockValidator) ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
	return validateDiskAttachmentsMock(diskAttachments)
}

func (v *mockValidator) ValidateNics(nics []*ovirtsdk.Nic) []validators.ValidationFailure {
	return validateNicsMock(nics)
}

func (v *mockValidator) ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1alpha1.NetworkResourceMappingItem, crNamespace string) []validators.ValidationFailure {
	return validateNetworkMappingsMock(nics, mapping, crNamespace)
}

func (v *mockValidator) ValidateDiskStatus(diskAttachment ovirtsdk.DiskAttachment) bool {
	return true
}

func (v *mockValidator) ValidateStorageMapping(
	attachments []*ovirtsdk.DiskAttachment,
	storageMapping *[]v2vv1alpha1.StorageResourceMappingItem,
	diskMapping *[]v2vv1alpha1.StorageResourceMappingItem,
) []validators.ValidationFailure {
	return validateStorageMappingMock(attachments, storageMapping, diskMapping)
}
