package validators

import (
	"fmt"

	v1 "k8s.io/api/storage/v1"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
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

// ValidateStorageMapping validates storage domain mapping and disk mapping
func (v *StorageMappingValidator) ValidateStorageMapping(
	attachments []*ovirtsdk.DiskAttachment,
	storageMapping *[]v2vv1alpha1.ResourceMappingItem,
	diskMapping *[]v2vv1alpha1.ResourceMappingItem,
) []ValidationFailure {
	var failures []ValidationFailure
	// Check whether mapping for storage is required and was provided
	if storageMapping == nil && diskMapping == nil && len(attachments) > 0 {
		failures = append(failures, ValidationFailure{
			ID:      StorageMappingID,
			Message: "Storage and Disk mappings are missing",
		})
		return failures
	}
	if storageMapping == nil {
		storageMapping = &[]v2vv1alpha1.ResourceMappingItem{}
	}
	if diskMapping == nil {
		diskMapping = &[]v2vv1alpha1.ResourceMappingItem{}
	}
	// requiredDomains holds the domains needed by disks that are not listed in diskMapping
	requiredDomains := v.getRequiredStorageDomains(attachments, diskMapping)
	requiredTargetsSet, sourceFailures := v.validateSourceStorageMapping(storageMapping, requiredDomains)
	failures = append(failures, sourceFailures...)

	storageFailures := v.validateTargetStorageMapping(requiredTargetsSet)
	failures = append(failures, storageFailures...)

	// check disk mappings for violations
	if len(*diskMapping) > 0 {
		disksByStorageClass, missingDiskFailures := v.validateSourcesDiskMapping(diskMapping, attachments)
		failures = append(failures, missingDiskFailures...)

		// validate disk mapping target storage classes
		targetFailures := v.validateTargetDiskMapping(disksByStorageClass)
		failures = append(failures, targetFailures...)
	}

	return failures
}

func (v *StorageMappingValidator) validateSourceStorageMapping(
	storageMapping *[]v2vv1alpha1.ResourceMappingItem,
	requiredDomains []v2vv1alpha1.Source,
) ([]v2vv1alpha1.ObjectIdentifier, []ValidationFailure) {
	var failures []ValidationFailure
	// Map storageMappings source id and name to ResourceMappingItem
	mapByID, mapByName := utils.IndexByIDAndName(storageMapping)
	requiredTargetsSet := make(map[v2vv1alpha1.ObjectIdentifier]bool)
	// Validate that all vm storage domains are mapped and populate requiredTargetsSet for target existence check
	for _, domain := range requiredDomains {
		if domain.ID != nil {
			if item, found := mapByID[*domain.ID]; found {
				requiredTargetsSet[item.Target] = true
				continue
			}
		}
		if domain.Name != nil {
			if item, found := mapByName[*domain.Name]; found {
				requiredTargetsSet[item.Target] = true
				continue
			}
		}
		failures = append(failures, ValidationFailure{
			ID:      StorageMappingID,
			Message: fmt.Sprintf("Required source storage domain '%s' lacks mapping", utils.ToLoggableID(domain.ID, domain.Name)),
		})
	}
	requiredTargets := []v2vv1alpha1.ObjectIdentifier{}
	for t := range requiredTargetsSet {
		requiredTargets = append(requiredTargets, t)
	}
	return requiredTargets, failures
}

func (v *StorageMappingValidator) validateTargetStorageMapping(requiredTargetsSet []v2vv1alpha1.ObjectIdentifier) []ValidationFailure {
	var failures []ValidationFailure
	for _, className := range requiredTargetsSet {
		if _, err := v.provider.Find(className.Name); err != nil {
			failures = append(failures, ValidationFailure{
				ID:      StorageTargetID,
				Message: fmt.Sprintf("Storage class %s has not been found. Error: %v", className.Name, err),
			})
		}
	}
	return failures
}

// validateSourcesDiskMapping reports failures for missing disks and returns a map of storage domain to disk
func (v *StorageMappingValidator) validateSourcesDiskMapping(
	diskMapping *[]v2vv1alpha1.ResourceMappingItem,
	attachments []*ovirtsdk.DiskAttachment,
) (map[string]*ovirtsdk.Disk, []ValidationFailure) {
	var failures []ValidationFailure
	diskByStorageClass := make(map[string]*ovirtsdk.Disk)
	disksByID, disksByName := v.getRequiredDisks(attachments)
	for _, mapping := range *diskMapping {
		if mapping.Source.ID != nil {
			if disk, ok := disksByID[*mapping.Source.ID]; ok {
				diskByStorageClass[mapping.Target.Name] = disk
				continue
			}
		}
		if mapping.Source.Name != nil {
			if disk, ok := disksByName[*mapping.Source.Name]; ok {
				diskByStorageClass[mapping.Target.Name] = disk
				continue
			}
		}
		failures = append(failures, ValidationFailure{
			ID:      DiskMappingID,
			Message: fmt.Sprintf("Source disk %s has not been found for the VM.", utils.ToLoggableID(mapping.Source.ID, mapping.Source.Name)),
		})
	}
	return diskByStorageClass, failures
}

func (v *StorageMappingValidator) validateTargetDiskMapping(disksByStorageClass map[string]*ovirtsdk.Disk) []ValidationFailure {
	var failures []ValidationFailure
	for className, disk := range disksByStorageClass {
		if _, err := v.provider.Find(className); err != nil {
			diskID, _ := disk.Id()
			failures = append(failures, ValidationFailure{
				ID:      DiskTargetID,
				Message: fmt.Sprintf("Storage class %s has not been found for disk %s. Error: %v", className, diskID, err),
			})
		}
	}
	return failures
}

func (v *StorageMappingValidator) getRequiredDisks(
	attachments []*ovirtsdk.DiskAttachment,
) (map[string]*ovirtsdk.Disk, map[string]*ovirtsdk.Disk) {
	disksByID := make(map[string]*ovirtsdk.Disk)
	disksByName := make(map[string]*ovirtsdk.Disk)
	for _, da := range attachments {
		if disk, ok := da.Disk(); ok {
			id, okID := disk.Id()
			if okID {
				disksByID[id] = disk
			}
			name, okName := disk.Alias()
			if okName {
				disksByName[name] = disk
			}
		}
	}
	return disksByID, disksByName
}

// getRequiredStorageDomains returns a set of required storage domains for storageMappings
func (v *StorageMappingValidator) getRequiredStorageDomains(
	attachments []*ovirtsdk.DiskAttachment,
	diskMapping *[]v2vv1alpha1.ResourceMappingItem,
) []v2vv1alpha1.Source {
	// Map diskMapping source id and name to ResourceMappingItem
	mapByID, mapByName := utils.IndexByIDAndName(diskMapping)
	storageMappingSourcesSet := make(map[v2vv1alpha1.Source]bool)
	for _, da := range attachments {
		if disk, ok := da.Disk(); ok {
			// skip storage domains for disks specified in diskMapping for later inspection
			diskID, _ := disk.Id()
			if _, ok := mapByID[diskID]; ok {
				continue
			}
			diskName, _ := disk.Alias()
			if _, ok := mapByName[diskName]; ok {
				continue
			}
			if sd, ok := disk.StorageDomain(); ok {
				if src, ok := createSourceStorageDomainIdentifier(sd); ok {
					storageMappingSourcesSet[*src] = true
				}
			}
		}
	}
	var sources []v2vv1alpha1.Source
	for source := range storageMappingSourcesSet {
		sources = append(sources, source)
	}
	return sources
}

func createSourceStorageDomainIdentifier(domain *ovirtsdk.StorageDomain) (*v2vv1alpha1.Source, bool) {
	id, okID := domain.Id()
	name, okName := domain.Name()
	if okID || okName {
		src := v2vv1alpha1.Source{
			ID:   &id,
			Name: &name}
		return &src, true
	}
	return nil, false
}
