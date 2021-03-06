package validators

import (
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"

	"github.com/kubevirt/vm-import-operator/pkg/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

// diskInterfaceOwner defines means of getting interface of a storage entity
type diskInterfaceOwner interface {
	Interface() (ovirtsdk.DiskInterface, bool)
}

// diskLogicalNameOwner defines means of getting logical name of a storage entity
type diskLogicalNameOwner interface {
	LogicalName() (string, bool)
}

// usesScsiReservationOwner defines means of getting uses scsi reservation flag value of a storage entity
type usesScsiReservationOwner interface {
	UsesScsiReservation() (bool, bool)
}

// ValidateDiskStatus return true if the disk status is valid:
func ValidateDiskStatus(diskAttachment ovirtsdk.DiskAttachment) bool {
	if disk, ok := diskAttachment.Disk(); ok {
		if status, ok := disk.Status(); ok && status != "ok" {
			return false
		}
	}

	return true
}

// ValidateDiskAttachments validates disk attachments
func ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []ValidationFailure {
	var failures []ValidationFailure
	if len(diskAttachments) == 0 {
		failures = append(failures, ValidationFailure{ID: DiskAttachmentsExistID, Message: "VM has no disks"})
		return failures
	}
	for _, da := range diskAttachments {
		failures = append(failures, validateDiskAttachment(da)...)
	}
	return failures
}

func validateDiskAttachment(diskAttachment *ovirtsdk.DiskAttachment) []ValidationFailure {
	var results []ValidationFailure
	var attachmentID = ""
	if id, ok := diskAttachment.Id(); ok {
		attachmentID = id
	}

	if failure, valid := isValidDiskAttachmentInterface(diskAttachment, attachmentID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskAttachmentLogicalName(diskAttachment, attachmentID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskAttachmentPassDiscard(diskAttachment, attachmentID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskAttachmentUsesScsiReservation(diskAttachment, attachmentID); !valid {
		results = append(results, failure)
	}
	if disk, ok := diskAttachment.Disk(); ok {
		results = append(results, validateDisk(disk)...)
	}
	return results
}

func validateDisk(disk *ovirtsdk.Disk) []ValidationFailure {
	var results []ValidationFailure
	var diskID = ""
	if id, ok := disk.Id(); ok {
		diskID = id
	}
	if failure, valid := isValidDiskInterface(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskLogicalName(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskUsesScsiReservation(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskBackup(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskLunStorage(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskPropagateErrors(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskWipeAfterDelete(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskStatus(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskStorageType(disk, diskID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDiskSgio(disk, diskID); !valid {
		results = append(results, failure)
	}

	return results
}

func isValidDiskAttachmentInterface(diskAttachment *ovirtsdk.DiskAttachment, attachmentID string) (ValidationFailure, bool) {
	return isValidStorageInterface(diskAttachment, attachmentID, DiskAttachmentInterfaceID)
}

func isValidDiskInterface(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	return isValidStorageInterface(disk, diskID, DiskInterfaceID)
}

func isValidStorageInterface(diskAttachment diskInterfaceOwner, ownerID string, checkID CheckID) (ValidationFailure, bool) {
	if iface, ok := diskAttachment.Interface(); ok {
		if _, found := mapper.DiskInterfaceModelMapping[string(iface)]; !found {
			return ValidationFailure{
				ID:      checkID,
				Message: fmt.Sprintf("%s %s uses interface %v. Allowed values: %v", checkID, ownerID, iface, utils.GetMapKeys(mapper.DiskInterfaceModelMapping)),
			}, false
		}
	}

	return ValidationFailure{}, true
}

func isValidDiskAttachmentLogicalName(diskAttachment *ovirtsdk.DiskAttachment, attachmentID string) (ValidationFailure, bool) {
	return isValidStorageLogicalName(diskAttachment, attachmentID, DiskAttachmentLogicalNameID)
}

func isValidDiskLogicalName(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	return isValidStorageLogicalName(disk, diskID, DiskLogicalNameID)
}

func isValidStorageLogicalName(logicalNameOwner diskLogicalNameOwner, ownerID string, checkID CheckID) (ValidationFailure, bool) {
	if logicalName, ok := logicalNameOwner.LogicalName(); ok {
		return ValidationFailure{
			ID:      checkID,
			Message: fmt.Sprintf("%s %s has logical name of %s defined", checkID, ownerID, logicalName),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskAttachmentPassDiscard(diskAttachment *ovirtsdk.DiskAttachment, attachmentID string) (ValidationFailure, bool) {
	if pd, ok := diskAttachment.PassDiscard(); ok && pd {
		return ValidationFailure{
			ID:      DiskAttachmentPassDiscardID,
			Message: fmt.Sprintf("disk attachment %s has pass_discard == true", attachmentID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskAttachmentUsesScsiReservation(diskAttachment *ovirtsdk.DiskAttachment, attachmentID string) (ValidationFailure, bool) {
	return isValidStorageUsesScsiReservation(diskAttachment, attachmentID, DiskAttachmentUsesScsiReservationID)
}

func isValidDiskUsesScsiReservation(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	return isValidStorageUsesScsiReservation(disk, diskID, DiskUsesScsiReservationID)
}

func isValidStorageUsesScsiReservation(owner usesScsiReservationOwner, ownerID string, checkID CheckID) (ValidationFailure, bool) {
	if sr, ok := owner.UsesScsiReservation(); ok && sr {
		return ValidationFailure{
			ID:      checkID,
			Message: fmt.Sprintf("%s %s has uses_scsi_reservation == true", checkID, ownerID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskBackup(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if backup, ok := disk.Backup(); ok && backup == "incremental" {
		return ValidationFailure{
			ID:      DiskBackupID,
			Message: fmt.Sprintf("disk %s uses backup == 'incremental'. Allowed value: 'none'.", diskID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskLunStorage(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if storage, ok := disk.LunStorage(); ok {
		var message string
		if id, ok := storage.Id(); ok {
			message = fmt.Sprintf("disk %s uses LUN storage with ID: %v", diskID, id)
		} else {
			message = fmt.Sprintf("disk %s uses LUN storage", diskID)
		}
		return ValidationFailure{
			ID:      DiskLunStorageID,
			Message: message,
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskPropagateErrors(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if propagate, ok := disk.PropagateErrors(); ok {
		return ValidationFailure{
			ID:      DiskPropagateErrorsID,
			Message: fmt.Sprintf("disk %s has propagate_errors configured: %t", diskID, propagate),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskWipeAfterDelete(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if enabled, ok := disk.WipeAfterDelete(); ok && enabled {
		return ValidationFailure{
			ID:      DiskWipeAfterDeleteID,
			Message: fmt.Sprintf("disk %s has wipe_after_delete enabled", diskID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskStatus(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if status, ok := disk.Status(); ok && status != "ok" {
		return ValidationFailure{
			ID:      DiskStatusID,
			Message: fmt.Sprintf("disk %s has illegal status: '%v'. Allowed value: 'ok'", diskID, status),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskStorageType(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if storageType, ok := disk.StorageType(); !ok || storageType != "image" {
		return ValidationFailure{
			ID:      DiskStoragaTypeID,
			Message: fmt.Sprintf("disk %s has illegal storage type: '%v'. Allowed value: 'image'", diskID, storageType),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDiskSgio(disk *ovirtsdk.Disk, diskID string) (ValidationFailure, bool) {
	if sgio, ok := disk.Sgio(); ok && sgio != "disabled" {
		return ValidationFailure{
			ID:      DiskSgioID,
			Message: fmt.Sprintf("disk %s has illegal sgio setting: '%v'. Allowed value: 'disabled'", diskID, sgio),
		}, false
	}
	return ValidationFailure{}, true
}
