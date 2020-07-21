package mappings

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
)

// MergeMappings creates new resource mapping spec containing all of the mappings from the externalMappingSpec with all mappings from crMappings. Mappings from the crMappings will overwrite ones from the external mapping.
func MergeMappings(externalMappingSpec *v2vv1alpha1.ResourceMappingSpec, vmiMapping *v2vv1alpha1.OvirtMappings) *v2vv1alpha1.OvirtMappings {
	if externalMappingSpec == nil && vmiMapping == nil {
		return &v2vv1alpha1.OvirtMappings{}
	}
	primaryMappings, secondaryMappings := extractMappings(externalMappingSpec, vmiMapping)

	networkMappings := mergeNetworkMappings(primaryMappings.NetworkMappings, secondaryMappings.NetworkMappings)
	storageMappings := mergeStorageMappings(primaryMappings.StorageMappings, secondaryMappings.StorageMappings)

	// diskMappings are expected to be provided only for a specific VM Import CR
	diskMappings := primaryMappings.DiskMappings

	ovirtMappings := v2vv1alpha1.OvirtMappings{
		NetworkMappings: networkMappings,
		StorageMappings: storageMappings,
		DiskMappings:    diskMappings,
	}
	return &ovirtMappings
}

func extractMappings(externalMappingSpec *v2vv1alpha1.ResourceMappingSpec, crMappings *v2vv1alpha1.OvirtMappings) (*v2vv1alpha1.OvirtMappings, *v2vv1alpha1.OvirtMappings) {
	var primaryMappings, secondaryMappings v2vv1alpha1.OvirtMappings
	if crMappings != nil {
		primaryMappings = *crMappings
	}

	if externalMappingSpec != nil && externalMappingSpec.OvirtMappings != nil {
		secondaryMappings = *externalMappingSpec.OvirtMappings
	}
	return &primaryMappings, &secondaryMappings
}

func mergeNetworkMappings(primaryMappings *[]v2vv1alpha1.NetworkResourceMappingItem, secondaryMappings *[]v2vv1alpha1.NetworkResourceMappingItem) *[]v2vv1alpha1.NetworkResourceMappingItem {
	var mapping []v2vv1alpha1.NetworkResourceMappingItem

	if primaryMappings == nil {
		return secondaryMappings
	}
	if secondaryMappings == nil {
		return primaryMappings
	}
	secondaryIDMap, secondaryNameMap := utils.IndexNetworkByIDAndName(secondaryMappings)
	usedIDs := make(map[string]bool)
	// Copy everything from the primary mapping to the output
	for _, item := range *primaryMappings {
		id := item.Source.ID
		name := item.Source.Name
		if id == nil && name == nil {
			continue
		}
		mapping = append(mapping, item)
		// Delete from the secondary what we've already used
		if id != nil {
			usedIDs[*id] = true
			delete(secondaryIDMap, *id)
		}
		if name != nil {
			delete(secondaryNameMap, *name)
		}
	}
	// Copy secondary items that we haven't used yet to the output
	for id, item := range secondaryIDMap {
		mapping = append(mapping, item)
		usedIDs[id] = true
		name := item.Source.Name
		if name != nil {
			delete(secondaryNameMap, *name)
		}
	}
	for _, item := range secondaryNameMap {
		if item.Source.ID == nil || !usedIDs[*item.Source.ID] {
			mapping = append(mapping, item)
		}
	}

	return &mapping
}

func mergeStorageMappings(primaryMappings *[]v2vv1alpha1.StorageResourceMappingItem, secondaryMappings *[]v2vv1alpha1.StorageResourceMappingItem) *[]v2vv1alpha1.StorageResourceMappingItem {
	var mapping []v2vv1alpha1.StorageResourceMappingItem

	if primaryMappings == nil {
		return secondaryMappings
	}
	if secondaryMappings == nil {
		return primaryMappings
	}
	secondaryIDMap, secondaryNameMap := utils.IndexStorageItemByIDAndName(secondaryMappings)
	usedIDs := make(map[string]bool)
	// Copy everything from the primary mapping to the output
	for _, item := range *primaryMappings {
		id := item.Source.ID
		name := item.Source.Name
		if id == nil && name == nil {
			continue
		}
		mapping = append(mapping, item)
		// Delete from the secondary what we've already used
		if id != nil {
			usedIDs[*id] = true
			delete(secondaryIDMap, *id)
		}
		if name != nil {
			delete(secondaryNameMap, *name)
		}
	}
	// Copy secondary items that we haven't used yet to the output
	for id, item := range secondaryIDMap {
		mapping = append(mapping, item)
		usedIDs[id] = true
		name := item.Source.Name
		if name != nil {
			delete(secondaryNameMap, *name)
		}
	}
	for _, item := range secondaryNameMap {
		if item.Source.ID == nil || !usedIDs[*item.Source.ID] {
			mapping = append(mapping, item)
		}
	}

	return &mapping
}
