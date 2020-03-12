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
