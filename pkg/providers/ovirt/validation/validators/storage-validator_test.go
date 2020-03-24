package validators

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

var _ = Describe("Validating Disk Attachment", func() {
	table.DescribeTable("should flag disk attachment with illegal interface: ", func(iface string) {
		var attachment = newDiskAttachment()
		attachment.SetInterface(ovirtsdk.DiskInterface(iface))

		failures := validateDiskAttachment(attachment)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskAttachmentInterfaceID))
	},
		table.Entry("ide", "pci"),
		table.Entry("spapr_vscsi", "spapr_vscsi"),

		table.Entry("garbage", "123saas-#$#@"),
		table.Entry("empty string", ""),
	)
	table.DescribeTable("should accept disk attachment with legal interface: ", func(iface string) {
		var attachment = newDiskAttachment()
		attachment.SetInterface(ovirtsdk.DiskInterface(iface))

		failures := validateDiskAttachment(attachment)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("virtio", "virtio"),
		table.Entry("sata", "sata"),
		table.Entry("virtio_scsi", "virtio_scsi"),
	)
	It("should flag disk attachment with logical name: ", func() {
		attachment := newDiskAttachment()
		attachment.SetLogicalName("/dev/sdMy")

		failures := validateDiskAttachment(attachment)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskAttachmentLogicalNameID))
	})
	It("should flag disk attachment with pass_discard == true: ", func() {
		attachment := newDiskAttachment()
		attachment.SetPassDiscard(true)

		failures := validateDiskAttachment(attachment)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskAttachmentPassDiscardID))
	})
	It("should flag disk attachment with uses_scsi_reservation == true: ", func() {
		attachment := newDiskAttachment()
		attachment.SetUsesScsiReservation(true)

		failures := validateDiskAttachment(attachment)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskAttachmentUsesScsiReservationID))
	})
})

func newDiskAttachment() *ovirtsdk.DiskAttachment {
	diskAttachment := ovirtsdk.DiskAttachment{}
	diskAttachment.SetId(fmt.Sprintf("ID_%d", rand.Int()))
	diskAttachment.SetInterface(ovirtsdk.DiskInterface("virtio"))
	diskAttachment.SetDisk(newDisk())
	return &diskAttachment
}

var _ = Describe("Validating Disk", func() {
	table.DescribeTable("should flag disk with illegal interface: ", func(iface string) {
		var disk = newDisk()
		disk.SetInterface(ovirtsdk.DiskInterface(iface))

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskInterfaceID))
	},
		table.Entry("ide", "ide"),
		table.Entry("spapr_vscsi", "spapr_vscsi"),

		table.Entry("garbage", "123saas-#$#@"),
		table.Entry("empty string", ""),
	)
	table.DescribeTable("should accept disk with legal interface: ", func(iface string) {
		var disk = newDisk()
		disk.SetInterface(ovirtsdk.DiskInterface(iface))

		failures := validateDisk(disk)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("virtio", "virtio"),
		table.Entry("sata", "sata"),
		table.Entry("virtio_scsi", "virtio_scsi"),
	)
	It("should flag disk with logical name: ", func() {
		disk := newDisk()
		disk.SetLogicalName("/dev/sdMy")

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskLogicalNameID))
	})
	It("should flag disk with uses_scsi_reservation == true: ", func() {
		disk := newDisk()
		disk.SetUsesScsiReservation(true)

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskUsesScsiReservationID))
	})
	It("should flag disk with backup == 'incremental': ", func() {
		disk := newDisk()
		disk.SetBackup(ovirtsdk.DiskBackup("incremental"))

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskBackupID))
	})
	It("should flag disk with lun_storage present: ", func() {
		disk := newDisk()
		lunStorage := ovirtsdk.HostStorage{}
		lunStorage.SetId("Lun_id")
		disk.SetLunStorage(&lunStorage)

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskLunStorageID))
	})
	It("should flag disk with propagate_errors == true setting present: ", func() {
		disk := newDisk()
		disk.SetPropagateErrors(true)

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskPropagateErrorsID))
	})
	It("should flag disk with propagate_errors == false setting present: ", func() {
		disk := newDisk()
		disk.SetPropagateErrors(false)

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskPropagateErrorsID))
	})
	It("should flag disk with wipe_after_delete == true setting present: ", func() {
		disk := newDisk()
		disk.SetWipeAfterDelete(true)

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskWipeAfterDeleteID))
	})
	table.DescribeTable("should flag disk with illegal status: ", func(status string) {
		var disk = newDisk()
		disk.SetStatus(ovirtsdk.DiskStatus(status))

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskStatusID))
	},
		table.Entry("illegal", "illegal"),
		table.Entry("locked", "locked"),

		table.Entry("garbage", "123saas-#$#@"),
		table.Entry("empty string", ""),
	)
	table.DescribeTable("should flag disk with illegal storage types: ", func(storageType string) {
		var disk = newDisk()
		disk.SetStorageType(ovirtsdk.DiskStorageType(storageType))

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskStoragaTypeID))
	},
		table.Entry("cinder", "cinder"),
		table.Entry("lun", "lun"),
		table.Entry("Managed block storage", "managed_block_storage"),

		table.Entry("garbage", "123saas-#$#@"),
		table.Entry("empty string", ""),
	)
	It("should flag disk without storage type: ", func() {
		disk := ovirtsdk.Disk{}
		disk.SetId("Disk_id")
		disk.SetInterface(ovirtsdk.DiskInterface("virtio"))

		failures := validateDisk(&disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskStoragaTypeID))
	})
	It("should allow for disk with disabled sgio: ", func() {
		disk := newDisk()
		disk.SetSgio(ovirtsdk.ScsiGenericIO("disabled"))

		failures := validateDisk(disk)

		Expect(failures).To(BeEmpty())
	})
	table.DescribeTable("should flag disk with illegal sgio settings: ", func(sgio string) {
		var disk = newDisk()
		disk.SetSgio(ovirtsdk.ScsiGenericIO(sgio))

		failures := validateDisk(disk)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(DiskSgioID))
	},
		table.Entry("Filtered", "filtered"),
		table.Entry("Unfiltered", "unfiltered"),
	)
})

func newDisk() *ovirtsdk.Disk {
	disk := ovirtsdk.Disk{}
	disk.SetId(fmt.Sprintf("ID_%d", rand.Int()))
	disk.SetInterface(ovirtsdk.DiskInterface("virtio"))
	disk.SetStatus("ok")
	disk.SetWipeAfterDelete(false)
	disk.SetBackup("none")
	disk.SetStorageType("image")
	return &disk
}
