package utils

import (
	"fmt"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
)

// GetMapKeys gets all keys from a map as a slice
func GetMapKeys(theMap map[string]string) []string {
	keys := make([]string, 0, len(theMap))

	for k := range theMap {
		keys = append(keys, k)
	}
	return keys
}

// ToLoggableResourceName creates loggable representation of maybe namespaced resource name
func ToLoggableResourceName(name string, namespace *string) string {
	identifier := name
	if namespace != nil {
		identifier = fmt.Sprintf("%s/%s", *namespace, name)
	}
	return identifier
}

//ToLoggableID creates loggable identifier that may be comprised of an id and/or a name
func ToLoggableID(id *string, name *string) string {
	var identifier string
	if id != nil {
		identifier = *id
	}
	if name != nil {
		if identifier != "" {
			identifier = fmt.Sprintf("(%s) ", identifier)
		}
		identifier = fmt.Sprintf("%s%s ", *name, identifier)
	}
	return identifier
}

// IndexByIDAndName indexes mapping array by ID and by Name
func IndexByIDAndName(mapping *[]v2vv1alpha1.ResourceMappingItem) (mapByID map[string]v2vv1alpha1.ResourceMappingItem, mapByName map[string]v2vv1alpha1.ResourceMappingItem) {
	mapByID = make(map[string]v2vv1alpha1.ResourceMappingItem)
	mapByName = make(map[string]v2vv1alpha1.ResourceMappingItem)
	for _, item := range *mapping {
		if item.Source.ID != nil {
			mapByID[*item.Source.ID] = item
		}
		if item.Source.Name != nil {
			mapByName[*item.Source.Name] = item
		}
	}
	return
}
