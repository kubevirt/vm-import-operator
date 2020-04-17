package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/alecthomas/units"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
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
			identifier = fmt.Sprintf("(%s)", identifier)
		}
		identifier = fmt.Sprintf("%s%s", *name, identifier)
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

// NormalizeName returns a normalized name based on the given name that complies to DNS 1123 format
func NormalizeName(name string) (string, error) {
	if len(name) == 0 {
		return "", fmt.Errorf("The provided name is empty")
	}
	errors := k8svalidation.IsDNS1123Subdomain(name)
	if len(errors) == 0 {
		return name, nil
	}

	// convert name to lowercase and replace '.' with '-'
	name = strings.ToLower(name)
	name = strings.Replace(name, ".", "-", -1)

	// slice string based on first and last alphanumeric character
	firstLegal := strings.IndexFunc(name, func(c rune) bool { return unicode.IsLower(c) || unicode.IsDigit(c) })
	lastLegal := strings.LastIndexFunc(name, func(c rune) bool { return unicode.IsLower(c) || unicode.IsDigit(c) })

	if firstLegal < 0 {
		return "", fmt.Errorf("The name doesn't contain a legal alphanumeric character")
	}

	name = name[firstLegal : lastLegal+1]
	reg := regexp.MustCompile("[^a-z0-9-]+")
	name = reg.ReplaceAllString(name, "")

	if len(name) > k8svalidation.DNS1123SubdomainMaxLength {
		name = name[0:k8svalidation.DNS1123SubdomainMaxLength]
	}
	return name, nil
}

// WithMessage joins message and newMessage with a ", " string and returns the resulting string or returns newMessage if message is empty
func WithMessage(message string, newMessage string) string {
	if message == "" {
		return newMessage
	}
	return fmt.Sprintf("%s, %s", message, newMessage)
}

// MakeLabelFrom creates label value from given namespace and name
func MakeLabelFrom(namespace string, name string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

// CountImportedDataVolumes return number of true values in map of booleans
func CountImportedDataVolumes(dvsDone map[string]bool) int {
	done := 0
	for _, isDone := range dvsDone {
		if isDone {
			done++
		}
	}

	return done
}

// FormatBytes convert bytes to highest suffix
func FormatBytes(bytes int64) (string, error) {
	if bytes < 0 {
		return "", fmt.Errorf("bytes can't be negative")
	}

	kib := int64(units.KiB)
	if bytes < kib {
		return fmt.Sprintf("%d", bytes), nil
	}
	suffix := 0
	n := int64(kib)
	for i := bytes / kib; i >= kib; i /= kib {
		n *= kib
		suffix++
	}
	// Return bytes in case we can't express exact number in highest suffix
	if bytes%n > 0 {
		return fmt.Sprintf("%d", bytes), nil
	}

	return fmt.Sprintf("%d%ci", bytes/n, "KMGTPE"[suffix]), nil
}

// IsUtcCompatible checks whether given timezone behaves like UTC - has the same offset of 0 and does not observer daylight saving time
func IsUtcCompatible(timezone string) bool {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false
	}
	now := time.Now().In(loc)
	_, offset := now.Zone()
	return offset == 0 && hasNoTimeChange(now)
}

func hasNoTimeChange(timeInTimezone time.Time) bool {
	location := timeInTimezone.Location()
	year := timeInTimezone.Year()
	_, winterOffset := time.Date(year, 1, 1, 0, 0, 0, 0, location).Zone()
	_, summerOffset := time.Date(year, 7, 1, 0, 0, 0, 0, location).Zone()
	return winterOffset == summerOffset
}
