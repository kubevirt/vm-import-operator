package vmware_test

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/tests"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

type networkedVMImportTest struct {
	framework *fwk.Framework
}

var _ = Describe("Networked VM import ", func() {
	var (
		f          = fwk.NewFrameworkOrDie("networked-vm-import", fwk.ProviderVmware)
		secret     corev1.Secret
		namespace  string
		test       = networkedVMImportTest{f}
		err        error
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name

		secret, err = f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
	})

	table.DescribeTable("should create started VM with pod network", func(networkType *string) {
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Vmware.Mappings = &v2vv1.VmwareMappings{
			NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: networkType},
			},
		}
		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeSuccessful(f))

		retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
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
	vmNamespace := t.framework.Namespace.Name
	vm, _ := t.framework.KubeVirtClient.VirtualMachine(vmNamespace).Get(vmName, &metav1.GetOptions{})
	spec := vm.Spec.Template.Spec

	By("having correct machine type")
	Expect(spec.Domain.Machine.Type).To(BeEquivalentTo("q35"))

	By("having EFI")
	Expect(spec.Domain.Firmware.Bootloader.BIOS).To(BeNil())
	Expect(spec.Domain.Firmware.Bootloader.EFI).ToNot(BeNil())

	By("having correct CPU configuration")
	cpu := spec.Domain.CPU
	Expect(cpu.Cores).To(BeEquivalentTo(1))
	Expect(cpu.Sockets).To(BeEquivalentTo(4))
	Expect(cpu.Threads).To(BeEquivalentTo(0))

	By("having correct network configuration")
	Expect(spec.Networks).To(HaveLen(1))
	Expect(spec.Networks[0].Pod).ToNot(BeNil())

	nic := spec.Domain.Devices.Interfaces[0]
	Expect(nic.Name).To(BeEquivalentTo(spec.Networks[0].Name))
	Expect(nic.MacAddress).To(BeEquivalentTo(vmware.VM70MacAddress))
	Expect(nic.Masquerade).ToNot(BeNil())
	Expect(nic.Model).To(BeEquivalentTo("virtio"))

	By("having correct clock settings")
	Expect(spec.Domain.Clock.UTC).ToNot(BeNil())

	By("having correct disk setup")
	disks := spec.Domain.Devices.Disks
	Expect(disks).To(HaveLen(2))
	disk0 := disks[0]
	Expect(disk0.Disk.Bus).To(BeEquivalentTo("virtio"))
	Expect(disk0.Name).To(BeEquivalentTo("dv-target-vm-disk-202-0"))
	disk1 := disks[1]
	Expect(disk1.Disk.Bus).To(BeEquivalentTo("virtio"))
	Expect(disk1.Name).To(BeEquivalentTo("dv-target-vm-disk-202-1"))

	By("having correct volumes")
	Expect(spec.Volumes).To(HaveLen(2))

	return vm
}
