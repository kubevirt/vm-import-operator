package ovirt_test

import (
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type vmValidationTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM validation ", func() {
	var (
		f          = fwk.NewFrameworkOrDie("vm-validation")
		secretName string
		test       = vmValidationTest{framework: f}
	)

	BeforeEach(func() {
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secretName = s.Name
	})

	table.DescribeTable("should block VM with unsupported status", func(status string) {
		vmID := vms.UnsupportedStatusVmIDPrefix + status
		vmXml := test.framework.LoadTemplate("invalid/vms/"+vms.UnsupportedStatusVmIDPrefix+"vm-template.xml", map[string]string{"@VMSTATUS": status})
		stubbing := test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
			Build()

		err := f.OvirtStubbingClient.Stub(stubbing)
		if err != nil {
			Fail(err.Error())
		}
		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("image_locked", "image_locked"),
		table.Entry("migrating", "migrating"),
		table.Entry("not_responding", "not_responding"),
		table.Entry("paused", "paused"),
		table.Entry("powering_down", "powering_down"),
		table.Entry("powering_up", "powering_up"),
		table.Entry("reboot_in_progress", "reboot_in_progress"),
		table.Entry("restoring_state", "restoring_state"),
		table.Entry("saving_state", "saving_state"),
		table.Entry("suspended", "suspended"),
		table.Entry("unassigned", "unassigned"),
		table.Entry("unknown", "unknown"),
		table.Entry("wait_for_launch", "wait_for_launch"),
	)

	table.DescribeTable("should block VM with", func(vmID string) {
		vmXml := test.framework.LoadFile("invalid/vms/" + vmID + "-vm.xml")
		stubbing := test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
			Build()

		err := f.OvirtStubbingClient.Stub(stubbing)
		if err != nil {
			Fail(err.Error())
		}
		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("unsupported i440fx_sea_bios BIOS type", vms.UnsupportedBiosTypeVmID),
		table.Entry("unsupported s390fx architecture", vms.UnsupportedArchitectureVmID),
		table.Entry("illegal images", vms.IlleagalImagesVmID),
		table.Entry("kubevirt origin", vms.KubevirtOriginVmID),
		table.Entry("placement policy affinity set to 'migratable'", vms.MigratablePlacementPolicyAffinityVmID),
		table.Entry("USB enabled", vms.UsbEnabledVmID),
	)

	It("should block VM with diag288 watchdog", func() {
		vmID := vms.UnsupportedDiag288WatchdogVmID
		vmXml := test.framework.LoadFile("invalid/vms/" + vmID + "-vm.xml")
		wdXml := test.framework.LoadFile("invalid/watchdogs/diag288.xml")
		stubbing := test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID+"/watchdogs", &wdXml).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
			Build()

		err := f.OvirtStubbingClient.Stub(stubbing)
		if err != nil {
			Fail(err.Error())
		}
		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})
})

func (t *vmValidationTest) prepareImport(vmID string, secretName string) *v2vv1alpha1.VirtualMachineImport {
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(vmID, namespace, secretName, t.framework.NsPrefix, true)
	created, err := t.framework.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)
	if err != nil {
		Fail(err.Error())
	}
	return created
}

func (t *vmValidationTest) stubResources(vmID string) *sapi.StubbingBuilder {
	nicsXml := t.framework.LoadFile("nics/empty.xml")
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/one.xml")
	diskXml := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXml := t.framework.LoadFile("graphic-consoles/empty.xml")
	return sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml)

}
