package validators

const (
	// NicInterfaceCheckID defines an ID of a NIC interface model check
	NicInterfaceCheckID = CheckID("nic.interface")
	// NicOnBootID defines an ID of a NIC on_boot == fales check
	NicOnBootID = CheckID("nic.on_boot")
	// NicPluggedID defines an ID of a NIC plugged == false check
	NicPluggedID = CheckID("nic.plugged")
	// NicVNicPortMirroringID defines an ID of a vnic_profile.port_mirroring == true check
	NicVNicPortMirroringID = CheckID("nic.vnic_profile.port_mirroring")
	// NicVNicPassThroughID defines an ID of a vnic_profile.pass_through == 'enabled' check
	NicVNicPassThroughID = CheckID("nic.vnic_profile.pass_through")
	// NicVNicCustomPropertiesID defines an ID of a vnic_profile.custom_properties presence check
	NicVNicCustomPropertiesID = CheckID("nic.vnic_profile.custom_properties")
	// NicVNicNetworkFilterID defines an ID of a vnic_profile.networ_filter presence check
	NicVNicNetworkFilterID = CheckID("nic.vnic_profile.network_filter")
	// NicVNicQosID defines an ID of a vnic_profile.qos presence check
	NicVNicQosID = CheckID("nic.vnic_profile.qos")
	// DiskAttachmentInterfaceID defines an ID of a disk attachment interface check
	DiskAttachmentInterfaceID = CheckID("disk_attachment.interface")
	// DiskAttachmentLogicalNameID defines an ID of a disk_attachment.logical_name check
	DiskAttachmentLogicalNameID = CheckID("disk_attachment.logical_name")
	// DiskAttachmentPassDiscardID defines an ID of a disk_attachment.pass_discard == true check
	DiskAttachmentPassDiscardID = CheckID("disk_attachment.pass_discard")
	// DiskAttachmentUsesScsiReservationID defines and ID of a disk_attachment.uses_scsi_reservation == true check
	DiskAttachmentUsesScsiReservationID = CheckID("disk_attachment.uses_scsi_reservation")
	// DiskInterfaceID defines an ID of a disk interface check
	DiskInterfaceID = CheckID("disk_attachment.disk.interface")
	// DiskLogicalNameID defines an ID of a disk.logical_name check
	DiskLogicalNameID = CheckID("disk_attachment.disk.logical_name")
	// DiskUsesScsiReservationID defines an ID of a disk.uses_scsi_reservation == true check
	DiskUsesScsiReservationID = CheckID("disk_attachment.disk.uses_scsi_reservation")
	// DiskBackupID defines an ID of a disk.backup == 'incremental' check
	DiskBackupID = CheckID("disk_attachment.disk.backup")
	// DiskLunStorageID defines an ID of a disk.lun_storage presence check
	DiskLunStorageID = CheckID("disk_attachment.disk.lun_storage")
	// DiskPropagateErrorsID defines an ID of a disk.propagate_errors presence check
	DiskPropagateErrorsID = CheckID("disk_attachment.disk.propagate_errors")
	// DiskWipeAfterDeleteID defines an ID of a disk.wipe_after_delete == true check
	DiskWipeAfterDeleteID = CheckID("disk_attachment.disk.wipe_after_delete")
	// DiskStatusID defines an ID of a disk.status == 'ok'
	DiskStatusID = CheckID("disk_attachment.disk.status")
	// DiskStoragaTypeID defines an ID of a disk.storage_type != image check
	DiskStoragaTypeID = CheckID("disk_attachment.disk.storage_type")
	// DiskSgioID defines an ID of a disk.sgio == true check
	DiskSgioID = CheckID("disk_attachment.disk.sgio")
	// VMBiosBootMenuID defines an ID of a vm.bios.boot_menu.enabled == true check
	VMBiosBootMenuID = CheckID("vm.bios.boot_menu.enabled")
	// VMBiosTypeID defines an ID of a vm.bios.type check
	VMBiosTypeID = CheckID("vm.bios.type")
	// VMBiosTypeQ35SecureBootID defines an ID of a vm.bios.type != q35_secure_boot check
	VMBiosTypeQ35SecureBootID = CheckID("vm.bios.type.q35_secure_boot")
	// VMCpuArchitectureID defines an ID of a vm.cpu.architecture != s390x check
	VMCpuArchitectureID = CheckID("vm.cpu.architecture")
	// VMCpuTuneID defines an ID of a vm.cpu.cpu_tune mapping check
	VMCpuTuneID = CheckID("vm.cpu.cpu_tune")
	// VMCpuSharesID defines an ID of a vm.cpu_shares check
	VMCpuSharesID = CheckID("vm.cpu_shares")
	// VMCustomPropertiesID defines an ID of a vm.custom_properties check
	VMCustomPropertiesID = CheckID("vm.custom_properties")
	// VMDisplayTypeID defines an ID of a vm.display.type == spice check
	VMDisplayTypeID = CheckID("vm.display.type")
	// VMHasIllegalImagesID defines an ID of a vm.has_illegal_images == true check
	VMHasIllegalImagesID = CheckID("vm.has_illegal_images")
	// VMHighAvailabilityPriorityID defines an ID of a vm.high_availability.priority check
	VMHighAvailabilityPriorityID = CheckID("vm.high_availability.priority")
	// VMIoThreadsID defines an ID of a vm.io.threads check
	VMIoThreadsID = CheckID("vm.io.threads")
	// VMMemoryPolicyBallooningID defines an ID of a vm.memory_policy.ballooning == true check
	VMMemoryPolicyBallooningID = CheckID("vm.memory_policy.ballooning")
	// VMMemoryPolicyOvercommitPercentID defines an ID of a vm.memory_policy.over_commit.percent check
	VMMemoryPolicyOvercommitPercentID = CheckID("vm.memory_policy.over_commit.percent")
	// VMMemoryPolicyGuaranteedID defines an ID of a vm.memory_policy.guaranteed check
	VMMemoryPolicyGuaranteedID = CheckID("vm.memory_policy.guaranteed")
	// VMMigrationID defines an ID of a vm.migration check
	VMMigrationID = CheckID("vm.migration")
	// VMMigrationDowntimeID defines an ID of a vm.migration_downtime check
	VMMigrationDowntimeID = CheckID("vm.migration_downtime")
	// VMNumaTuneModeID defines an ID of a vm.numa_tune_mode check
	VMNumaTuneModeID = CheckID("vm.numa_tune_mode")
	// VMOriginID defines an ID of a vm.origin == kubevirt check
	VMOriginID = CheckID("vm.origin")
	// VMRngDeviceSourceID defines an ID of a vm.rng_device.source != urandom check
	VMRngDeviceSourceID = CheckID("vm.rng_device.source")
	// VMSoundcardEnabledID defines an ID of a vm.soundcard_enabled == true check
	VMSoundcardEnabledID = CheckID("vm.soundcard_enabled")
	// VMStartPausedID defines an ID of a vm.start_paused == true check
	VMStartPausedID = CheckID("vm.start_paused")
	// VMStorageErrorResumeBehaviourID defines an ID of a vm.storage_error_resume_behaviour check
	VMStorageErrorResumeBehaviourID = CheckID("vm.storage_error_resume_behaviour")
	// VMTunnelMigrationID defines an ID of a vm.tunnel_migration == true check
	VMTunnelMigrationID = CheckID("vm.tunnel_migration")
	// VMUsbID defines an ID of a vm.usb check
	VMUsbID = CheckID("vm.usb")
	// VMGraphicConsolesID defines an ID of a vm.graphic_consoles.protocol == spice check
	VMGraphicConsolesID = CheckID("vm.graphic_consoles.protocol")
	// VMHostDevicesID defines an ID of a vm.host_devices check
	VMHostDevicesID = CheckID("vm.host_devices")
	// VMReportedDevicesID defines an ID of a vm.reported_devices check
	VMReportedDevicesID = CheckID("vm.reported_devices")
	// VMQuotaID defines an ID of a vm.quota check
	VMQuotaID = CheckID("vm.quota")
	// VMWatchdogsID defines an ID of a vm.watchdogs
	VMWatchdogsID = CheckID("vm.watchdogs")
	// VMCdromsID defines an ID of a vm.cdroms storage domain type check
	VMCdromsID = CheckID("vm.cdroms.file.storage_domain.type")
	// VMFloppiesID defines an ID of a vm.floppies presence check
	VMFloppiesID = CheckID("vm.floppies")
	// NetworkMappingID defines an ID of a check verifying that all the required source networks are present in the resource mapping
	NetworkMappingID = CheckID("network.mapping")
	// NetworkTypeID defines an ID of a check verifying supported network types
	NetworkTypeID = CheckID("network.type")
	// NetworkTargetID defines an ID of a check verifyting existence of target network
	NetworkTargetID = CheckID("network.target")
	// StorageMappingID defines an ID of a check verifying that all the required source storage domains are present in the resource mapping
	StorageMappingID = CheckID("storage.mapping")
	// StorageTargetID defines an ID of a check verifying existence of target storage class
	StorageTargetID = CheckID("storage.target")
	// DiskMappingID defines an ID of a check verifying that all the required source disks are present in the resource mapping
	DiskMappingID = CheckID("disk.mapping")
	// DiskTargetID defines an ID of a check verifying existence of target storage class
	DiskTargetID = CheckID("disk.target")
)

// CheckID identifies validation check for Virtual Machine Import
type CheckID string

// ValidationFailure describes Virtual Machine Import validation failure
type ValidationFailure struct {
	// Check ID
	ID CheckID
	// Verbose explanation of the failure
	Message string
}
