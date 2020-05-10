package tests_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	"github.com/onsi/ginkgo/extensions/table"

	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

type basicVmImportTest struct {
	framework *fwk.Framework
}

var _ = Describe("Basic VM import ", func() {
	var (
		vmID = "123"

		diskID           = "123"
		storageDomainsID = "123"

		storageClass = "local"
	)

	var (
		f    = fwk.NewFrameworkOrDie("basic-vm-import")
		test = basicVmImportTest{
			framework: f,
		}
		secret    corev1.Secret
		namespace string
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	Context(" without resource mapping", func() {
		It("should create stopped VM", func() {
			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, false)

			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).NotTo(BeRunning(f))

			vm := test.validateTargetConfiguration(vmBlueprint.Name)
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))
		})

		It("should create started VM", func() {
			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)

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

	Context(" with in-CR resource mapping", func() {
		table.DescribeTable("should create running VM", func(mappings v2vv1alpha1.OvirtMappings, storageClass string) {
			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Ovirt.Mappings = &mappings

			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			vm := test.validateTargetConfiguration(vmBlueprint.Name)
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveStorageClass(storageClass, f))
		},
			table.Entry(" for disk", v2vv1alpha1.OvirtMappings{
				DiskMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &diskID}, Target: v2vv1alpha1.ObjectIdentifier{Name: storageClass}},
				},
			}, storageClass),
			table.Entry(" for storage domain", v2vv1alpha1.OvirtMappings{
				StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &storageDomainsID}, Target: v2vv1alpha1.ObjectIdentifier{Name: storageClass}},
				},
			}, storageClass))
	})
})

func (t *basicVmImportTest) validateTargetConfiguration(vmName string) *v1.VirtualMachine {
	vmNamespace := t.framework.Namespace.Name
	f := t.framework
	vm, _ := f.KubeVirtClient.VirtualMachine(vmNamespace).Get(vmName, &metav1.GetOptions{})
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

	By("having no network")
	Expect(spec.Networks).To(BeEmpty())
	Expect(spec.Domain.Devices.Interfaces).To(BeEmpty())

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

	By("having correct VNC setup")
	Expect(*spec.Domain.Devices.AutoattachGraphicsDevice).To(BeTrue())

	inputDevices := spec.Domain.Devices.Inputs
	tablet := utils.FindTablet(inputDevices)
	Expect(tablet).NotTo(BeNil())
	Expect(tablet.Bus).To(BeEquivalentTo("virtio"))
	Expect(tablet.Type).To(BeEquivalentTo("tablet"))

	return vm
}
