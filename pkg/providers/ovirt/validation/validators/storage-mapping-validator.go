package validators

import (
	"fmt"

	v1 "k8s.io/api/storage/v1"
	"kubevirt.io/client-go/kubecli"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
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

// NewStorageMappingValidator creates new StorageMappingValidator that will use provided KubevirtClient
func NewStorageMappingValidator(kubevirt kubecli.KubevirtClient) StorageMappingValidator {
	return StorageMappingValidator{
		provider: &StorageClasses{
			Client: kubevirt.StorageV1().StorageClasses(),
		},
	}
}

// ValidateStorageMapping validates storage domain mapping
func (v *StorageMappingValidator) ValidateStorageMapping(attachments []*ovirtsdk.DiskAttachment, mapping *[]v2vv1alpha1.ResourceMappingItem) []ValidationFailure {
	var failures []ValidationFailure
	// Check whether mapping for storage is required and was provided
	if mapping == nil && len(attachments) > 0 {
		failures = append(failures, ValidationFailure{
			ID:      StorageMappingID,
			Message: "Storage mapping is missing",
		})
		return failures
	}
	requiredDomains := v.getRequiredStorageDomains(attachments)
	// Map source id and name to ResourceMappingItem
	mapByID, mapByName := IndexByIDAndName(mapping)

	requiredTargetsSet := make(map[v2vv1alpha1.ObjectIdentifier]bool)
	// Validate that all vm storage domains are mapped and populate requiredTargetsSet for target existence check
	for _, domain := range requiredDomains {
		if domain.ID != nil {
			item, found := mapByID[*domain.ID]
			if found {
				requiredTargetsSet[item.Target] = true
				continue
			}
		}
		if domain.Name != nil {
			item, found := mapByName[*domain.Name]
			if found {
				requiredTargetsSet[item.Target] = true
				continue
			}
		}
		failures = append(failures, ValidationFailure{
			ID:      StorageMappingID,
			Message: fmt.Sprintf("Required source storage domain '%s' lacks mapping", ToLoggableID(domain.ID, domain.Name)),
		})
	}

	for className := range requiredTargetsSet {
		_, err := v.provider.Find(className.Name)
		if err != nil {
			failures = append(failures, ValidationFailure{
				ID:      StorageTargetID,
				Message: fmt.Sprintf("Storage class %s has not been found. Error: %v", className.Name, err),
			})
		}
	}
	return failures
}

func (v *StorageMappingValidator) getRequiredStorageDomains(attachments []*ovirtsdk.DiskAttachment) []v2vv1alpha1.Source {
	sourcesSet := make(map[v2vv1alpha1.Source]bool)
	for _, da := range attachments {
		if disk, ok := da.Disk(); ok {
			if sd, ok := disk.StorageDomain(); ok {
				if src, ok := v.createSourceStorageDomainIdentifier(sd); ok {
					sourcesSet[*src] = true
				}
			}
		}
	}
	var sources []v2vv1alpha1.Source
	for source := range sourcesSet {
		sources = append(sources, source)
	}
	return sources
}

func (v *StorageMappingValidator) createSourceStorageDomainIdentifier(domain *ovirtsdk.StorageDomain) (*v2vv1alpha1.Source, bool) {
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
