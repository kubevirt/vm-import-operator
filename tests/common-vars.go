package tests

var (
	// StorageClass defines the name of the storage class available in the test cluster
	StorageClass = "local"

	// TrueVar defines variable with value of `true`
	TrueVar = true

	// PodType defines `pod` resource mapping network type
	PodType = "pod"
	// MultusType defines `pod` resource mapping network type
	MultusType = "multus"
	// UnsupportedType defines non-existing, unsupported `unsupported resource mapping network type
	UnsupportedType = "unsupported"
)
