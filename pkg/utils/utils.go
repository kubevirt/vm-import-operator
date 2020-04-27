package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/alecthomas/units"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	// MaxLabelValueLength specifies max length of k8s label value
	MaxLabelValueLength = 63
)

var (
	log = logf.Log.WithName("utils")
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

// MakeLabelFrom creates label value from given CR name
func MakeLabelFrom(name string) string {
	n := len(name)
	if n > MaxLabelValueLength {
		log.Info("Label value will be shortened to 63 characters", "CR name", name)
		suffix := strconv.Itoa(n)
		suffixLen := len(suffix)
		maxNameLen := MaxLabelValueLength - suffixLen - 1
		return fmt.Sprintf("%s_%s", name[:maxNameLen], suffix)
	}
	return name
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
	// Africa/El_Aaiun is off DST during Ramadan - a non-fixed date period
	if location.String() == "Africa/El_Aaiun" {
		return false
	}
	year := timeInTimezone.Year()
	_, winterOffset := time.Date(year, 1, 1, 0, 0, 0, 0, location).Zone()
	_, summerOffset := time.Date(year, 7, 1, 0, 0, 0, 0, location).Zone()
	return winterOffset == summerOffset
}

// ParseUtcOffsetToSeconds parses UTC offset string ([+|-]HH:MM) into representation in seconds. Only syntactical validation is performed on input string.
func ParseUtcOffsetToSeconds(offset string) (int, error) {
	if len(offset) != 6 {
		return 0, fmt.Errorf("utc offset string has illegal length: %s", offset)
	}
	sign := offset[0]
	multiplier := 0
	switch sign {
	case '+':
		multiplier = 1
	case '-':
		multiplier = -1
	default:
		return 0, fmt.Errorf("utc offset string does not start with a sign character: %s", offset)
	}
	parts := strings.Split(offset[1:], ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("utc offset string is malformed: %s", offset)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("utc offset hours segment is malformed: %s", offset)
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("utc offset minutes segment is malformed: %s", offset)
	}

	return multiplier * 60 * (hours*60 + minutes), nil
}
