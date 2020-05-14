package validators

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/config"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("validators")

// ValidatorWrapper exposes validator package API as a struct
type ValidatorWrapper struct {
	networkMappingValidator NetworkMappingValidator
	storageMappingValidator StorageMappingValidator
	kvConfigProvider        config.KubeVirtConfigProvider
}

// NewValidatorWrapper creates new, configured ValidatorWrapper
func NewValidatorWrapper(client client.Client, kvConfigProvider config.KubeVirtConfigProvider) *ValidatorWrapper {
	netAttachDefProvider := NetworkAttachmentDefinitions{
		Client: client,
	}
	storageClassesProvider := StorageClasses{
		Client: client,
	}
	return &ValidatorWrapper{
		networkMappingValidator: NewNetworkMappingValidator(&netAttachDefProvider),
		storageMappingValidator: NewStorageMappingValidator(&storageClassesProvider),
		kvConfigProvider:        kvConfigProvider,
	}
}

// ValidateVM wraps validators package implementation of ValidateVM function
func (v *ValidatorWrapper) ValidateVM(vm *ovirtsdk.Vm) []ValidationFailure {
	kvConfig, err := v.kvConfigProvider.GetConfig()
	if err != nil {
		logger.Error(err, "Cannot get KubeVirt cluster config.")
	}
	return ValidateVM(vm, kvConfig)
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
func (v *ValidatorWrapper) ValidateStorageMapping(
	attachments []*ovirtsdk.DiskAttachment,
	storageMapping *[]v2vv1alpha1.ResourceMappingItem,
	diskMappings *[]v2vv1alpha1.ResourceMappingItem,
) []ValidationFailure {
	return v.storageMappingValidator.ValidateStorageMapping(attachments, storageMapping, diskMappings)
}
