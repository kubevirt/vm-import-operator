package vms

const (
	BasicVmID                                    = "123"
	TwoDisksVmID                                 = "two-disks"
	InvalidNicInterfaceVmIDPrefix                = "nic-interface-"
	UnsupportedStatusVmIDPrefix                  = "unsupported-status-"
	UnsupportedBiosTypeVmID                      = "unsupported-i440fx_sea_bios-bios-type"
	UnsupportedArchitectureVmID                  = "unsupported-s390x-architecture"
	IlleagalImagesVmID                           = "illegal-images"
	KubevirtOriginVmID                           = "kubevirt-origin"
	MigratablePlacementPolicyAffinityVmID        = "migratable-placement-policy-affinity"
	UsbEnabledVmID                               = "usb-enabled"
	UnsupportedDiag288WatchdogVmID               = "unsupported-diag288-watchdog"
	BasicNetworkVmID                             = "basic-network"
	TwoNetworksVmID                              = "two-networks"
	UnsupportedDiskAttachmentInterfaceVmIDPrefix = "unsupported-disk-attachment-interface-"
	UnsupportedDiskInterfaceVmIDPrefix           = "unsupported-disk-interface-"
	ScsiReservationDiskAttachmentVmID            = "scsi-reservation-disk-attachment"
	ScsiReservationDiskVmID                      = "scsi-reservation-disk"
	LUNStorageDiskVmID                           = "lun-storage-disk"
	IllegalDiskStatusVmIDPrefix                  = "illegal-disk-status-"
	UnsupportedDiskStorageTypeVmIDPrefix         = "unsupported-disk-storage-type-"
	UnsupportedDiskSGIOVmIDPrefix                = "unsupported-disk-sgio-type-"
)

var (
	VirtioDiskID    = "123"
	StorageDomainID = "123"

	BasicNetworkID = "123"

	VNicProfile1ID = "vnic-profile-1"
	VNicProfile2ID = "vnic-profile-2"

	BasicNetworkVmNicMAC = "56:6f:05:0f:00:05"
	Nic2MAC              = "56:6f:05:0f:00:06"
)
