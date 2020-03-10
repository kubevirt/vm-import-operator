package admission

import (
	"k8s.io/api/admission/v1beta1"
)

type action int

const (
	log   = 0
	warn  = 1
	block = 2
)

var checkToAction = map[CheckID]action{
	// NIC rules
	NicInterfaceCheckID:       block,
	NicOnBootID:               log,
	NicPluggedID:              warn,
	NicVNicPassThroughID:      block,
	NicVNicPortMirroringID:    warn,
	NicVNicCustomPropertiesID: warn,
	NicVNicNetworkFilterID:    warn,
	NicVNicQosID:              log,
	// Storage rules
	DiskAttachmentInterfaceID:           block,
	DiskAttachmentLogicalNameID:         log,
	DiskAttachmentPassDiscardID:         log,
	DiskAttachmentUsesScsiReservationID: block,
	DiskInterfaceID:                     block,
	DiskLogicalNameID:                   log,
	DiskUsesScsiReservationID:           block,
	DiskBackupID:                        warn,
	DiskLunStorageID:                    block,
	DiskPropagateErrorsID:               log,
	DiskWipeAfterDeleteID:               log,
	DiskStatusID:                        block,
	DiskStoragaTypeID:                   block,
	DiskSgioID:                          block,
	// VM rules
	VMBiosBootMenuID:                  log,
	VMBiosTypeID:                      block,
	VMBiosTypeQ35SecureBootID:         warn,
	VMCpuArchitectureID:               block,
	VMCpuTuneID:                       warn,
	VMCpuSharesID:                     log,
	VMCustomPropertiesID:              warn,
	VMDisplayTypeID:                   log,
	VMHasIllegalImagesID:              block,
	VMHighAvailabilityPriorityID:      log,
	VMIoThreadsID:                     warn,
	VMMemoryPolicyBallooningID:        log,
	VMMemoryPolicyOvercommitPercentID: log,
	VMMemoryPolicyGuaranteedID:        log,
	VMMigrationID:                     log,
	VMMigrationDowntimeID:             log,
	VMNumaTuneModeID:                  warn,
	VMOriginID:                        block,
	VMRngDeviceSourceID:               log,
	VMSoundcardEnabledID:              warn,
	VMStartPausedID:                   log,
	VMStorageErrorResumeBehaviourID:   log,
	VMTunnelMigrationID:               warn,
	VMUsbID:                           block,
	VMGraphicConsolesID:               log,
	VMHostDevicesID:                   log,
	VMReportedDevicesID:               log,
	VMQuotaID:                         log,
	VMWatchdogsID:                     block,
	VMCdromsID:                        log,
	VMFloppiesID:                      log,
}

// VirtualMachineImportAdmitter validates VirtualMachineImport object
type VirtualMachineImportAdmitter struct {
}

// Admit validates whether VM described in VirtualMachineImport can be imported
func (admitter *VirtualMachineImportAdmitter) Admit(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}
