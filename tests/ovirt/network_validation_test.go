package ovirt_test

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/tests"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type networkValidationTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM network validation ", func() {
	var (
		f          = fwk.NewFrameworkOrDie("network-validation")
		secretName string
		test       = networkValidationTest{framework: f}
	)

	BeforeEach(func() {
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secretName = s.Name
	})

	table.DescribeTable("should block VM with unsupported NIC interface", func(iFace string) {
		vmID := vms.InvalidNicInterfaceVmIDPrefix + iFace
		nicsXml := f.LoadTemplate("nics/interface-template.xml", map[string]string{"@INTERFACE": iFace})
		vnicProfileXml := f.LoadFile("vnic-profiles/vnic-profile-1.xml")
		test.stub(vmID, &nicsXml, &vnicProfileXml)

		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("rtl8139_virtio", "rtl8139_virtio"),
		table.Entry("spapr_vlan", "spapr_vlan"),
	)

	It("should not block VM with pass-through enabled in the vnic profile", func() {
		vmID := vms.NicPassthroughVmID
		nicsXml := f.LoadFile("nics/one.xml")
		vnicProfileXml := f.LoadFile("vnic-profiles/pass-through.xml")
		test.stub(vmID, &nicsXml, &vnicProfileXml)
		created := test.prepareImport(vmID, secretName)

		Expect(created).To(HaveMappingRulesVerified(f))
	})
})

func (t *networkValidationTest) prepareImport(vmID string, secretName string) *v2vv1.VirtualMachineImport {
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(vmID, namespace, secretName, t.framework.NsPrefix, true)
	vmi.Spec.Source.Ovirt.Mappings = &v2vv1.OvirtMappings{
		NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
			{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType},
		},
	}
	created, err := t.framework.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)
	if err != nil {
		Fail(err.Error())
	}
	return created
}

func (t *networkValidationTest) stub(vmID string, nicsXml *string, vnicProfileXml *string) {
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/one.xml")
	diskXml := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXml := t.framework.LoadFile("graphic-consoles/empty.xml")
	networkXml := t.framework.LoadFile("networks/net-1.xml")
	vmXml := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", nicsXml).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXml).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXml).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", vnicProfileXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
