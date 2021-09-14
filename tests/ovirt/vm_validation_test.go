package ovirt_test

import (
	"context"
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"
	"github.com/onsi/ginkgo/extensions/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type vmValidationTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM validation ", func() {
	var (
		f          = fwk.NewFrameworkOrDie("vm-validation", fwk.ProviderOvirt)
		secretName string
		test       = vmValidationTest{framework: f}
	)

	BeforeEach(func() {
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secretName = s.Name
	})

	table.DescribeTable("should block VM with unsupported status", func(status string) {
		vmID := vms.UnsupportedStatusVmIDPrefix + status
		vmXML := test.framework.LoadTemplate("vms/status-template.xml", map[string]string{"@VMSTATUS": status, "@VMID": vmID})
		stubbing := test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML).
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

	table.DescribeTable("should block VM with", func(vmID string, vmFile string, vmMacros map[string]string) {
		vmMacros["@VMID"] = vmID
		vmXML := test.framework.LoadTemplate("vms/"+vmFile, vmMacros)
		stubbing := test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML).
			Build()

		err := f.OvirtStubbingClient.Stub(stubbing)
		if err != nil {
			Fail(err.Error())
		}
		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("unsupported s390x architecture", vms.UnsupportedArchitectureVmID, "architecture-template.xml", map[string]string{"@ARCH": "s390x"}),
		table.Entry("USB enabled", vms.UsbEnabledVmID, "usb-template.xml", map[string]string{"@ENABLED": "true"}),
		table.Entry("placement policy affinity set to 'migratable'", vms.MigratablePlacementPolicyAffinityVmID, "placement-policy-affinity-template.xml", map[string]string{"@AFFINITY": "migratable"}),
		table.Entry("kubevirt origin", vms.KubevirtOriginVmID, "origin-template.xml", map[string]string{"@ORIGIN": "kubevirt"}),
		table.Entry("illegal images", vms.IlleagalImagesVmID, "has-illegal-images-template.xml", map[string]string{"@ILLEGALIMAGES": "true"}),
	)

	It("should block VM with diag288 watchdog", func() {
		vmID := vms.UnsupportedDiag288WatchdogVmID
		vmXML := test.framework.LoadTemplate("vms/watchdog-vm.xml", map[string]string{"@VMID": vmID})
		wdXML := test.framework.LoadTemplate("watchdogs/model-template.xml", map[string]string{"@MODEL": "diag288"})
		stubbing := test.stubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID+"/watchdogs", &wdXML).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML).
			Build()

		err := f.OvirtStubbingClient.Stub(stubbing)
		if err != nil {
			Fail(err.Error())
		}
		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})
})

func (t *vmValidationTest) prepareImport(vmID string, secretName string) *v2vv1.VirtualMachineImport {
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secretName, t.framework.NsPrefix, true)
	created, err := t.framework.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})
	if err != nil {
		Fail(err.Error())
	}
	return created
}

func (t *vmValidationTest) stubResources(vmID string) *sapi.StubbingBuilder {
	nicsXML := t.framework.LoadFile("nics/empty.xml")
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "50331648"})
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXML := t.framework.LoadFile("graphic-consoles/empty.xml")
	return sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXML).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML)
}
