package ovirt_test

import (
	"context"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/client-go/api/v1"
)

type networkedVMImportTest struct {
	framework *fwk.Framework
}

var _ = Describe("Networked VM import ", func() {
	var (
		f         = fwk.NewFrameworkOrDie("networked-vm-import", fwk.ProviderOvirt)
		secret    corev1.Secret
		namespace string
		test      = networkedVMImportTest{framework: f}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	table.DescribeTable("should create started VM with pod network", func(networkType *string) {
		vmID := vms.BasicVmID
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Ovirt.Mappings = &v2vv1.OvirtMappings{
			NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: networkType},
			},
		}
		test.stub(vmID)
		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeSuccessful(f))

		retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
		Expect(vmBlueprint).To(BeRunning(f))

		vm := test.validateTargetConfiguration(vmBlueprint.Name)
		Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))
	},
		table.Entry("when type in network resource mapping is 'pod'", &tests.PodType),
		table.Entry("when type in network resource mapping is missing (nil)", nil),
	)
})

func (t *networkedVMImportTest) validateTargetConfiguration(vmName string) *v1.VirtualMachine {
	vmNamespacedName := types.NamespacedName{
		Namespace: t.framework.Namespace.Name,
		Name: vmName,
	}
	vm := &v1.VirtualMachine{}
	_ = t.framework.Client.Get(context.TODO(), vmNamespacedName, vm)
	spec := vm.Spec.Template.Spec

	By("having correct machine type")
	Expect(spec.Domain.Machine.Type).To(BeEquivalentTo("q35"))

	By("having BIOS")
	Expect(spec.Domain.Firmware.Bootloader.BIOS).NotTo(BeNil())
	Expect(spec.Domain.Firmware.Bootloader.EFI).To(BeNil())

	By("having correct CPU configuration")
	cpu := spec.Domain.CPU
	Expect(cpu.Cores).To(BeEquivalentTo(1))
	Expect(cpu.Sockets).To(BeEquivalentTo(1))
	Expect(cpu.Threads).To(BeEquivalentTo(1))

	By("having correct network configuration")
	Expect(spec.Networks).To(HaveLen(1))
	Expect(spec.Networks[0].Pod).ToNot(BeNil())

	nic := spec.Domain.Devices.Interfaces[0]
	Expect(nic.Name).To(BeEquivalentTo(spec.Networks[0].Name))
	Expect(nic.MacAddress).To(BeEquivalentTo(vms.BasicNetworkVmNicMAC))
	Expect(nic.Masquerade).ToNot(BeNil())
	Expect(nic.Model).To(BeEquivalentTo("virtio"))

	By("having correct clock settings")
	Expect(spec.Domain.Clock.Timezone).ToNot(BeNil())

	By("having correct disk setup")
	disks := spec.Domain.Devices.Disks
	Expect(disks).To(HaveLen(1))
	disk1 := disks[0]
	Expect(disk1.Disk.Bus).To(BeEquivalentTo("virtio"))
	Expect(*disk1.BootOrder).To(BeEquivalentTo(1))

	By("having correct volumes")
	Expect(spec.Volumes).To(HaveLen(1))

	return vm
}

func (t *networkedVMImportTest) stub(vmID string) {
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "50331648"})
	consolesXML := t.framework.LoadFile("graphic-consoles/vnc.xml")
	networkXML := t.framework.LoadFile("networks/net-1.xml")
	vnicProfileXML := t.framework.LoadFile("vnic-profiles/vnic-profile-1.xml")
	vmXML := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	nicsXML := t.framework.LoadFile("nics/one.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXML).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXML).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", &vnicProfileXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
