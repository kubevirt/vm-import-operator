package admission

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
	// DiskSgioID defines and ID of a disk.sgio == true check
	DiskSgioID = CheckID("disk_attachment.disk.sgio")
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
