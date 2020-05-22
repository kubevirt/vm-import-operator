package ovirt_vms

const (
	BasicVmID                             = "123"
	TwoDisksVmID                          = "two-disks"
	InvalidNicInterfaceVmIDPrefix         = "nic-interface-"
	UnsupportedStatusVmIDPrefix           = "unsupported-status-"
	UnsupportedBiosTypeVmID               = "unsupported-i440fx_sea_bios-bios-type"
	UnsupportedArchitectureVmID           = "unsupported-s390x-architecture"
	IlleagalImagesVmID                    = "illegal-images"
	KubevirtOriginVmID                    = "kubevirt-origin"
	MigratablePlacementPolicyAffinityVmID = "migratable-placement-policy-affinity"
	UsbEnabledVmID                        = "usb-enabled"
	UnsupportedDiag288WatchdogVmID        = "unsupported-diag288-watchdog"
	BasicNetworkVmID                      = "basic-network"
)

var (
	VirtioDiskID    = "123"
	StorageDomainID = "123"

	BasicNetworkID = "123"

	BasicNetworkVmNicMAC = "56:6f:05:0f:00:05"
)
