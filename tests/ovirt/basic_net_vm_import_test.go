package ovirt_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/tests"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

type networkedVmImportTest struct {
	framework *fwk.Framework
}

var _ = Describe("Networked VM import ", func() {
	var (
		f         = fwk.NewFrameworkOrDie("networked-vm-import")
		secret    corev1.Secret
		namespace string
		test      = networkedVmImportTest{framework: f}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	It("should create started VM", func() {
		vmID := vms.BasicVmID
		vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
			NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
				{Source: v2vv1alpha1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType},
			},
		}
		test.stub(vmID)
		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeSuccessful(f))

		retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
		Expect(vmBlueprint).To(BeRunning(f))

		vm := test.validateTargetConfiguration(vmBlueprint.Name)
		Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))
	})
})

func (t *networkedVmImportTest) validateTargetConfiguration(vmName string) *v1.VirtualMachine {
	vmNamespace := t.framework.Namespace.Name
	vm, _ := t.framework.KubeVirtClient.VirtualMachine(vmNamespace).Get(vmName, &metav1.GetOptions{})
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
	Expect(spec.Domain.Clock.UTC).ToNot(BeNil())

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

func (t *networkedVmImportTest) stub(vmID string) {
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/one.xml")
	diskXml := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXml := t.framework.LoadFile("graphic-consoles/vnc.xml")
	networkXml := t.framework.LoadFile("networks/net-1.xml")
	vnicProfileXml := t.framework.LoadFile("vnic-profiles/vnic-profile-1.xml")
	vmXml := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	nicsXml := t.framework.LoadFile("nics/one.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXml).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXml).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", &vnicProfileXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
