package utils

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	k8stypes "k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/client-go/api/v1"

	"github.com/alecthomas/units"
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// RestoreVMStateFinalizer defines restore source vm finalizer
	RestoreVMStateFinalizer = "vmimport.v2v.kubevirt.io/restore-state"

	// CleanupSnapshotsFinalizer defines a finalizer to remove warm import snapshots
	CleanupSnapshotsFinalizer = "vmimport.v2v.kubevirt.io/cleanup-snapshots"

	// Finalaizer for handling cancelled import
	CancelledImportFinalizer = "vmimport.v2v.kubevirt.io/cancelled-import"
)

var (
	// windowsUtcCompatibleTimeZones defines Windows-specific UTC-compatible timezones
	windowsUtcCompatibleTimeZones = map[string]bool{
		"GMT Standard Time":       true,
		"Greenwich Standard Time": true,
	}
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

// IndexStorageItemByIDAndName indexes mapping array by ID and by Name
func IndexStorageItemByIDAndName(mapping *[]v2vv1.StorageResourceMappingItem) (mapByID map[string]v2vv1.StorageResourceMappingItem, mapByName map[string]v2vv1.StorageResourceMappingItem) {
	mapByID = make(map[string]v2vv1.StorageResourceMappingItem)
	mapByName = make(map[string]v2vv1.StorageResourceMappingItem)
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

// IndexNetworkByIDAndName indexes mapping array by ID and by Name
func IndexNetworkByIDAndName(mapping *[]v2vv1.NetworkResourceMappingItem) (mapByID map[string]v2vv1.NetworkResourceMappingItem, mapByName map[string]v2vv1.NetworkResourceMappingItem) {
	mapByID = make(map[string]v2vv1.NetworkResourceMappingItem)
	mapByName = make(map[string]v2vv1.NetworkResourceMappingItem)
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

	name, err := normalize(name)
	if err != nil {
		return name, err
	}

	if len(name) > k8svalidation.DNS1123SubdomainMaxLength {
		name = name[0:k8svalidation.DNS1123SubdomainMaxLength]
	}
	return name, nil
}

func normalize(name string) (string, error) {
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

	return name, nil
}

// NormalizeLabel returns a normalized label based on the given name that complies to DNS 1123 format
func NormalizeLabel(name string) (string, error) {
	if len(name) == 0 {
		return "", fmt.Errorf("The provided label is empty")
	}
	errors := k8svalidation.IsDNS1123Label(name)
	if len(errors) == 0 {
		return name, nil
	}

	name, err := normalize(name)
	if err != nil {
		return name, err
	}

	return EnsureLabelValueLength(name), nil
}

// WithMessage joins message and newMessage with a ", " string and returns the resulting string or returns newMessage if message is empty
func WithMessage(message string, newMessage string) string {
	if message == "" {
		return newMessage
	}
	return fmt.Sprintf("%s, %s", message, newMessage)
}

// EnsureLabelValueLength shortens given value to the maximum label value length of 63 characters
func EnsureLabelValueLength(value string) string {
	n := len(value)
	if n > k8svalidation.LabelValueMaxLength {
		suffix := strconv.Itoa(n)
		suffixLen := len(suffix)
		maxNameLen := k8svalidation.LabelValueMaxLength - suffixLen - 1
		return fmt.Sprintf("%s-%s", value[:maxNameLen], suffix)
	}
	return value
}

// AppendMap adds a map
func AppendMap(origin map[string]string, newMap map[string]string) {
	for key, value := range newMap {
		origin[key] = value
	}
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
	if windowsUtcCompatibleTimeZones[timezone] {
		return true
	}
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

// AddFinalizer adds finalizer to VM import CR
func AddFinalizer(cr *v2vv1.VirtualMachineImport, name string, client rclient.Client) error {
	copy := cr.DeepCopy()
	if HasFinalizer(copy, name) {
		return nil
	}
	copy.Finalizers = append(copy.Finalizers, name)

	patch := rclient.MergeFrom(cr)
	err := client.Patch(context.TODO(), copy, patch)

	if err == nil {
		cr.Finalizers = copy.Finalizers
	}

	return err
}

// HasFinalizer checks whether specific finalizer is set on VM import CR
func HasFinalizer(cr *v2vv1.VirtualMachineImport, name string) bool {
	for _, f := range cr.GetFinalizers() {
		if f == name {
			return true
		}
	}
	return false
}

// RemoveFinalizer removes specific finalizer from VM import CR
func RemoveFinalizer(cr *v2vv1.VirtualMachineImport, name string, client rclient.Client) error {
	copy := cr.DeepCopy()
	if !HasFinalizer(copy, name) {
		return nil
	}

	var finalizers []string
	for _, f := range copy.Finalizers {
		if f != name {
			finalizers = append(finalizers, f)
		}
	}
	copy.Finalizers = finalizers

	patch := rclient.MergeFrom(cr)
	err := client.Patch(context.TODO(), copy, patch)

	if err == nil {
		cr.Finalizers = copy.Finalizers
	}

	return err
}

// FoldCleanUpErrors combines clean up errors into one error
func FoldCleanUpErrors(errs []error, vmiName k8stypes.NamespacedName) error {
	message := ""
	for _, e := range errs {
		message = WithMessage(message, e.Error())
	}
	return fmt.Errorf("clean-up for %v failed: %s", ToLoggableResourceName(vmiName.Name, &vmiName.Namespace), message)
}

// UpdateLabels updates a VirtualMachine's labels with values from a provided map, overwriting any duplicates
func UpdateLabels(vm *v1.VirtualMachine, labels map[string]string) {
	AppendMap(vm.ObjectMeta.GetLabels(), labels)
	AppendMap(vm.Spec.Template.ObjectMeta.GetLabels(), labels)
}

// UpdateAnnotations updates a VirtualMachine's annotations with values from a provided map, overwriting any duplicates
func UpdateAnnotations(vm *v1.VirtualMachine, annotationMap map[string]string) {
	annotations := vm.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		vm.ObjectMeta.SetAnnotations(annotations)
	}
	AppendMap(annotations, annotationMap)
}

func GetOverheadForStorageClass(filesystemOverhead cdiv1.FilesystemOverhead, storageClass *string) float64 {
	scName := ""
	defaultOverhead := 0.055

	if storageClass != nil {
		scName = *storageClass
	}

	rawOverhead, ok := filesystemOverhead.StorageClass[scName]
	if !ok {
		rawOverhead = filesystemOverhead.Global
	}
	overhead, err := strconv.ParseFloat(string(rawOverhead), 64)
	if err != nil {
		overhead = defaultOverhead
	}
	return overhead
}
