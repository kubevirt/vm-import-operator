package ovirt_test

import (
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type storageValidationTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM storage validation ", func() {
	var (
		f          = fwk.NewFrameworkOrDie("storage-validation")
		secretName string
		test       = storageValidationTest{framework: f}
	)

	BeforeEach(func() {
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secretName = s.Name
	})

	It("should block VM with no disk attachments", func() {
		vmID := vms.BasicNetworkVmID
		created := test.prepareImport(vmID, secretName)
		diskAttachmentsXml := f.LoadFile("invalid/disk-attachments/disk-attachments-empty.xml")
		vmXml := f.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
		f.OvirtStubbingClient.Stub(test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
			Build())

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})

	table.DescribeTable("should block VM with unsupported disk attachment interface", func(iFace string) {
		vmID := vms.UnsupportedDiskAttachmentInterfaceVmIDPrefix + iFace
		created := test.prepareImport(vmID, secretName)
		diskAttachmentsXml := f.LoadTemplate("/disk-attachments/interface-template.xml", map[string]string{"@INTERFACE": iFace})
		diskXml := f.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
		err := test.stubForDiskAttachments(vmID, &diskAttachmentsXml, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("ide", "ide"),
		table.Entry("spapr_vscsi", "spapr_vscsi"),
	)

	It("should block VM with disk attachment with SCSI reservation", func() {
		vmID := vms.ScsiReservationDiskAttachmentVmID
		created := test.prepareImport(vmID, secretName)

		diskAttachmentsXml := f.LoadFile("/invalid/disk-attachments/scsi-reservation.xml")
		diskXml := f.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
		err := test.stubForDiskAttachments(vmID, &diskAttachmentsXml, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})

	table.DescribeTable("should block VM with unsupported disk interface", func(iFace string) {
		vmID := vms.UnsupportedDiskInterfaceVmIDPrefix + iFace
		created := test.prepareImport(vmID, secretName)

		diskXml := f.LoadTemplate("disks/interface-template.xml", map[string]string{"@DISKSIZE": "46137344", "@INTERFACE": iFace})
		err := test.stubForDisks(vmID, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("ide", "ide"),
		table.Entry("spapr_vscsi", "spapr_vscsi"),
	)

	It("should block VM with disk with SCSI reservation", func() {
		vmID := vms.ScsiReservationDiskVmID
		created := test.prepareImport(vmID, secretName)

		diskXml := f.LoadTemplate("invalid/disks/scsi-reservation.xml", map[string]string{"@DISKSIZE": "46137344"})
		err := test.stubForDisks(vmID, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})

	It("should block VM with LUN storage", func() {
		vmID := vms.LUNStorageDiskVmID
		created := test.prepareImport(vmID, secretName)

		diskXml := f.LoadTemplate("invalid/disks/lun-storage.xml", map[string]string{"@DISKSIZE": "46137344"})
		err := test.stubForDisks(vmID, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})
	table.DescribeTable("should block VM with illegal disk status", func(status string) {
		vmID := vms.IllegalDiskStatusVmIDPrefix + status
		created := test.prepareImport(vmID, secretName)

		diskXml := f.LoadTemplate("disks/status-template.xml", map[string]string{"@DISKSIZE": "46137344", "@STATUS": status})
		err := test.stubForDisks(vmID, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("locked", "locked"),
		table.Entry("illegal", "illegal"),
	)

	table.DescribeTable("should block VM with unsupported disk storage type", func(storageType string) {
		vmID := vms.UnsupportedDiskStorageTypeVmIDPrefix + storageType
		created := test.prepareImport(vmID, secretName)

		diskXml := f.LoadTemplate("disks/storage-type-template.xml", map[string]string{"@DISKSIZE": "46137344", "@STORAGETYPE": storageType})
		err := test.stubForDisks(vmID, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("cinder", "cinder"),
		table.Entry("managed_block_storage", "managed_block_storage"),
		table.Entry("lun", "lun"),
	)

	table.DescribeTable("should block VM with unsupported SGIO setting", func(sgio string) {
		vmID := vms.UnsupportedDiskSGIOVmIDPrefix + sgio

		created := test.prepareImport(vmID, secretName)
		diskXml := f.LoadTemplate("disks/sgio-template.xml", map[string]string{"@DISKSIZE": "46137344", "@SGIO": sgio})
		err := test.stubForDisks(vmID, &diskXml)
		if err != nil {
			Fail(err.Error())
		}

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("filtered", "filtered"),
		table.Entry("unfiltered", "unfiltered"),
	)
})

func (t *storageValidationTest) prepareImport(vmID string, secretName string) *v2vv1.VirtualMachineImport {
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(vmID, namespace, secretName, t.framework.NsPrefix, true)
	created, err := t.framework.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)
	if err != nil {
		Fail(err.Error())
	}
	return created
}

func (t *storageValidationTest) stubForDiskAttachments(vmID string, diskAttachmentsXml *string, diskXml *string) error {
	vmXml := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	return t.framework.OvirtStubbingClient.Stub(t.stubResources(vmID).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", diskAttachmentsXml).
		StubGet("/ovirt-engine/api/disks/disk-1", diskXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
		Build())
}

func (t *storageValidationTest) stubForDisks(vmID string, diskXml *string) error {
	vmXml := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/one.xml")
	return t.framework.OvirtStubbingClient.Stub(t.stubResources(vmID).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/disks/disk-1", diskXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
		Build())
}

func (t *storageValidationTest) stubResources(vmID string) *sapi.StubbingBuilder {
	nicsXml := t.framework.LoadFile("nics/empty.xml")
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXml := t.framework.LoadFile("graphic-consoles/empty.xml")
	return sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml)

}
