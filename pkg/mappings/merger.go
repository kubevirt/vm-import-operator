package mappings

import (
	"github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
)

func MergeNetworkMappings(primaryMappings *[]v1beta1.NetworkResourceMappingItem, secondaryMappings *[]v1beta1.NetworkResourceMappingItem) *[]v1beta1.NetworkResourceMappingItem {
	var mapping []v1beta1.NetworkResourceMappingItem

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

func MergeStorageMappings(primaryMappings *[]v1beta1.StorageResourceMappingItem, secondaryMappings *[]v1beta1.StorageResourceMappingItem) *[]v1beta1.StorageResourceMappingItem {
	var mapping []v1beta1.StorageResourceMappingItem

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


