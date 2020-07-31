package validators

import (
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"

	v1 "k8s.io/api/storage/v1"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

// StorageClassProvider retrieves StorageClass for given name
type StorageClassProvider interface {
	Find(name string) (*v1.StorageClass, error)
}

// StorageMappingValidator provides storage mappings validation logic
type StorageMappingValidator struct {
	provider StorageClassProvider
}

// NewStorageMappingValidator creates new StorageMappingValidator that will use given provider
func NewStorageMappingValidator(provider StorageClassProvider) StorageMappingValidator {
	return StorageMappingValidator{
		provider: provider,
	}
}

type sourceType string

const (
	diskSourceType          = sourceType("disk")
	storageDomainSourceType = sourceType("storage domain")
)

var (
	defaultStorageClassTarget = v2vv1.ObjectIdentifier{Name: mapper.DefaultStorageClassTargetName}
)

type mappingSource struct {
	ID   *string
	Name *string
	Type sourceType
}

// ValidateStorageMapping validates storage domain mapping and disk mapping
func (v *StorageMappingValidator) ValidateStorageMapping(
	attachments []*ovirtsdk.DiskAttachment,
	storageMapping *[]v2vv1.StorageResourceMappingItem,
	diskMapping *[]v2vv1.StorageResourceMappingItem,
) []ValidationFailure {
	if storageMapping == nil {
		storageMapping = &[]v2vv1.StorageResourceMappingItem{}
	}
	if diskMapping == nil {
		diskMapping = &[]v2vv1.StorageResourceMappingItem{}
	}
	// requiredTargetsSet holds the storage classes required for mapping
	requiredTargetsSet := v.getRequiredStorageClasses(attachments, diskMapping, storageMapping)
	storageFailures := v.validateStorageClasses(requiredTargetsSet)

	return storageFailures
}

// getRequiredStorageClasses returns a set of required storage classes mapped to source description
func (v *StorageMappingValidator) getRequiredStorageClasses(
	attachments []*ovirtsdk.DiskAttachment,
	diskMapping *[]v2vv1.StorageResourceMappingItem,
	storageMapping *[]v2vv1.StorageResourceMappingItem,
) map[v2vv1.ObjectIdentifier][]mappingSource {
	// Map diskMapping source id and name to ResourceMappingItem
	mapDiskByID, mapDiskByName := utils.IndexStorageItemByIDAndName(diskMapping)
	mapDomainByID, mapDomainByName := utils.IndexStorageItemByIDAndName(storageMapping)
	storageMappingTargetSet := make(map[v2vv1.ObjectIdentifier][]mappingSource)
	for _, da := range attachments {
		if disk, ok := da.Disk(); ok {
			diskID, _ := disk.Id()
			if mapping, ok := mapDiskByID[diskID]; ok {
				source := mappingSource{ID: mapping.Source.ID, Name: mapping.Source.Name, Type: diskSourceType}
				storageMappingTargetSet[mapping.Target] = append(storageMappingTargetSet[mapping.Target], source)
				continue
			}
			diskName, _ := disk.Alias()
			if mapping, ok := mapDiskByName[diskName]; ok {
				source := mappingSource{ID: mapping.Source.ID, Name: mapping.Source.Name, Type: diskSourceType}
				storageMappingTargetSet[mapping.Target] = append(storageMappingTargetSet[mapping.Target], source)
				continue
			}
			if sd, ok := disk.StorageDomain(); ok {
				id, _ := sd.Id()
				if mapping, ok := mapDomainByID[id]; ok {
					source := mappingSource{ID: mapping.Source.ID, Name: mapping.Source.Name, Type: storageDomainSourceType}
					storageMappingTargetSet[mapping.Target] = append(storageMappingTargetSet[mapping.Target], source)
					continue
				}
				name, _ := sd.Name()
				if mapping, ok := mapDomainByName[name]; ok {
					source := mappingSource{ID: mapping.Source.ID, Name: mapping.Source.Name, Type: storageDomainSourceType}
					storageMappingTargetSet[mapping.Target] = append(storageMappingTargetSet[mapping.Target], source)
					continue
				}
			}
			// Add default storage class target
			source := mappingSource{ID: &diskID, Name: &diskName, Type: diskSourceType}
			storageMappingTargetSet[defaultStorageClassTarget] = append(storageMappingTargetSet[defaultStorageClassTarget], source)
		}
	}
	return storageMappingTargetSet
}

func (v *StorageMappingValidator) validateStorageClasses(requiredTargetsSet map[v2vv1.ObjectIdentifier][]mappingSource) []ValidationFailure {
	var failures []ValidationFailure
	for className, sources := range requiredTargetsSet {
		if className.Name == mapper.DefaultStorageClassTargetName {
			for _, source := range sources {
				resourceName := utils.ToLoggableID(source.ID, source.Name)
				failures = append(failures, ValidationFailure{
					ID:      StorageTargetDefaultClass,
					Message: fmt.Sprintf("Default storage class will be used for %s disk ", resourceName),
				})
			}
			// allow for forcing default storage class later on
			continue
		}
		if _, err := v.provider.Find(className.Name); err != nil {
			for _, source := range sources {
				checkID := getCheckID(source)
				resourceName := utils.ToLoggableID(source.ID, source.Name)
				failures = append(failures, ValidationFailure{
					ID:      checkID,
					Message: fmt.Sprintf("Storage class %s has not been found for %v: %s. Error: %v", className.Name, source.Type, resourceName, err),
				})
			}
		}
	}
	return failures
}

func getCheckID(source mappingSource) CheckID {
	switch source.Type {
	case diskSourceType:
		return DiskTargetID
	case storageDomainSourceType:
		fallthrough
	default:
		return StorageTargetID
	}
}
