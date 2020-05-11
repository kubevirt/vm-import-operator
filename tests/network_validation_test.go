package tests_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	vms "github.com/kubevirt/vm-import-operator/tests/ovirt-vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
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
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secretName = s.Name
	})

	table.DescribeTable("should block VM with unsupported NIC interface", func(iFace string) {
		created := test.prepareImport(vms.InvalidNicInterfaceVmIDPrefix+iFace, secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	},
		table.Entry("pci_passthrough", "pci_passthrough"),
		table.Entry("rtl8139_virtio", "rtl8139_virtio"),
		table.Entry("spapr_vlan", "spapr_vlan"),
	)

	It("should block VM with pass-through enabled in the vnic profile", func() {
		created := test.prepareImport("nic-passthrough", secretName)

		Expect(created).To(HaveMappingRulesVerificationFailure(f))
	})
})

func (t *networkValidationTest) prepareImport(vmID string, secretName string) *v2vv1alpha1.VirtualMachineImport {
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(vmID, namespace, secretName, t.framework.NsPrefix, true)
	vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
		NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
			{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &podType},
		},
	}
	created, err := t.framework.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)
	if err != nil {
		Fail(err.Error())
	}
	return created
}
