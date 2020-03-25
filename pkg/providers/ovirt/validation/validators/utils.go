package validators

import "fmt"

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
