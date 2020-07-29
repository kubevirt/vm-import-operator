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
	corev1 "k8s.io/api/core/v1"
)

type resourceMappingValidationTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM import ", func() {

	var (
		f         = fwk.NewFrameworkOrDie("resource-mapping-validation")
		secret    corev1.Secret
		namespace string
		test      = resourceMappingValidationTest{f}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	table.DescribeTable("should block VM import with", func(networkMappings *[]v2vv1.NetworkResourceMappingItem, storageMappings *[]v2vv1.StorageResourceMappingItem, diskMapping *[]v2vv1.StorageResourceMappingItem) {
		vmID := vms.BasicNetworkVmID
		test.stub(vmID)
		vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Ovirt.Mappings = &v2vv1.OvirtMappings{
			NetworkMappings: networkMappings,
			StorageMappings: storageMappings,
			DiskMappings:    diskMapping,
		}
		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.IncompleteMappingRules)))
	},
		table.Entry("missing network mapping",
			nil,
			nil,
			nil),
		table.Entry("network mapping to non-existing target network",
			&[]v2vv1.NetworkResourceMappingItem{{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.MultusType, Target: v2vv1.ObjectIdentifier{Name: "no-such-net-attach-def"}}},
			nil,
			nil),
		table.Entry("network mapping to unsupported target type",
			&[]v2vv1.NetworkResourceMappingItem{{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.UnsupportedType, Target: v2vv1.ObjectIdentifier{Name: "some-network"}}},
			nil,
			nil),
		table.Entry("network mapping to missing type and target with a namespace",
			&[]v2vv1.NetworkResourceMappingItem{{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: nil, Target: v2vv1.ObjectIdentifier{Name: "some-network", Namespace: &namespace}}},
			nil,
			nil),
		table.Entry("storage domain mapping to non-existing target storage class",
			&[]v2vv1.NetworkResourceMappingItem{{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType}},
			&[]v2vv1.StorageResourceMappingItem{{Source: v2vv1.Source{ID: &vms.StorageDomainID}, Target: v2vv1.ObjectIdentifier{Name: "no-such-storage-class"}}},
			nil),
		table.Entry("disk mapping to non-existing target storage class",
			&[]v2vv1.NetworkResourceMappingItem{{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType}},
			nil,
			&[]v2vv1.StorageResourceMappingItem{{Source: v2vv1.Source{ID: &vms.DiskID}, Target: v2vv1.ObjectIdentifier{Name: "no-such-storage-class"}}}),
	)

	It("should block import with two networks mapped to a pod network", func() {
		vmID := vms.TwoNetworksVmID
		vmXML := f.LoadFile("vms/two-networks-vm.xml")
		nicsXML := f.LoadFile("nics/two.xml")
		network2XML := f.LoadFile("networks/net-2.xml")
		vnicProfile2XML := f.LoadFile("vnic-profiles/vnic-profile-2.xml")
		stubbing := test.prepareCommonSubResources(vmID).
			StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML).
			StubGet("/ovirt-engine/api/networks/net-2", &network2XML).
			StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-2", &vnicProfile2XML).
			Build()
		err := f.OvirtStubbingClient.Stub(stubbing)
		Expect(err).To(BeNil())

		vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Ovirt.Mappings = &v2vv1.OvirtMappings{
			NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
				{
					Source: v2vv1.Source{ID: &vms.VNicProfile1ID},
					Type:   &tests.PodType,
					Target: v2vv1.ObjectIdentifier{
						Name: "network1",
					}},
				{
					Source: v2vv1.Source{ID: &vms.VNicProfile2ID},
					Type:   &tests.PodType,
					Target: v2vv1.ObjectIdentifier{
						Name: "network2",
					}},
			},
		}
		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.IncompleteMappingRules)))
	})
})

func (t *resourceMappingValidationTest) stub(vmID string) {
	nicsXML := t.framework.LoadFile("nics/one.xml")
	vmXML := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	builder := t.prepareCommonSubResources(vmID).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}

func (t *resourceMappingValidationTest) prepareCommonSubResources(vmID string) *sapi.StubbingBuilder {
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXML := t.framework.LoadFile("graphic-consoles/vnc.xml")
	networkXML := t.framework.LoadFile("networks/net-1.xml")
	vnicProfileXML := t.framework.LoadFile("vnic-profiles/vnic-profile-1.xml")
	return sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXML).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXML).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", &vnicProfileXML)
}
