package tests_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

var (
	// ID of a VM existing in oVirt described by the secret.
	vmID = "123"

	trueVar = true

	diskID           = "123"
	storageDomainsID = "123"

	storageClass = "local"
)

var _ = Describe("Basic VM import ", func() {
	var (
		f         = framework.NewFrameworkOrDie("basic-vm-import")
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
			vmi := virtualMachineImportCr(vmID, namespace, secret.Name)

			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).NotTo(BeRunning(f))

			vm := validateTargetMachinConfiguration(f, vmBlueprint.Name, vmBlueprint.Namespace)
			validateDefaultStorageClassWasRequested(f, vm.Spec.Template.Spec.Volumes[0].DataVolume.Name)
		})

		It("should create started VM", func() {
			vmi := virtualMachineImportCr(vmID, namespace, secret.Name)
			vmi.Spec.StartVM = &trueVar

			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			vm := validateTargetMachinConfiguration(f, vmBlueprint.Name, vmBlueprint.Namespace)
			validateDefaultStorageClassWasRequested(f, vm.Spec.Template.Spec.Volumes[0].DataVolume.Name)
		})
	})

	Context(" with in-CR resource mapping", func() {
		table.DescribeTable("should create running VM", func(mappings v2vv1alpha1.OvirtMappings, storageClass string) {
			vmi := virtualMachineImportCr(vmID, namespace, secret.Name)
			vmi.Spec.StartVM = &trueVar
			vmi.Spec.Source.Ovirt.Mappings = &mappings

			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			vm := validateTargetMachinConfiguration(f, vmBlueprint.Name, vmBlueprint.Namespace)
			validateVolumeStorageClass(f, vm.Spec.Template.Spec.Volumes[0].DataVolume.Name, &storageClass)
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

func validateVolumeStorageClass(f *framework.Framework, dvName string, storageClass *string) {
	dv, err := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dvName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(dv.Spec.PVC.StorageClassName).To(BeEquivalentTo(storageClass))
}

func validateDefaultStorageClassWasRequested(f *framework.Framework, dvName string) {
	validateVolumeStorageClass(f, dvName, nil)
}

func validateTargetMachinConfiguration(f *framework.Framework, vmName string, vmNamespace string) *v1.VirtualMachine {
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

	return vm
}

func virtualMachineImportCr(vmID string, namespace string, ovirtSecretName string) v2vv1alpha1.VirtualMachineImport {
	targetVMName := "target-vm"
	return v2vv1alpha1.VirtualMachineImport{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vm-import-basic-no-resources-",
			Namespace:    namespace,
		},
		Spec: v2vv1alpha1.VirtualMachineImportSpec{
			ProviderCredentialsSecret: v2vv1alpha1.ObjectIdentifier{
				Name:      ovirtSecretName,
				Namespace: &namespace,
			},
			Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
				Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{
					VM: v2vv1alpha1.VirtualMachineImportOvirtSourceVMSpec{ID: &vmID},
				},
			},
			TargetVMName: &targetVMName,
		},
	}
}
