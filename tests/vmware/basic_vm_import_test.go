package vmware_test

import (
	"context"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
	"github.com/onsi/ginkgo/extensions/table"
	"k8s.io/apimachinery/pkg/types"
	"time"

	"github.com/kubevirt/vm-import-operator/pkg/conditions"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
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
		f         = fwk.NewFrameworkOrDie("basic-vm-import", fwk.ProviderVmware)
		secret    corev1.Secret
		namespace string
		test      = basicVmImportTest{f}
		err       error
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name

		secret, err = f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
	})

	Context(" without resource mapping", func() {
		It("should create stopped VM", func() {
			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, false)

			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			validCondition := conditions.FindConditionOfType(retrieved.Status.Conditions, v2vv1.Valid)
			Expect(validCondition.Status).To(BeEquivalentTo(corev1.ConditionTrue))
			Expect(*validCondition.Reason).To(BeEquivalentTo(v2vv1.ValidationCompleted))

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).NotTo(BeRunning(f).Timeout(2 * time.Minute))

			vm := test.validateTargetConfiguration(vmBlueprint.Name)
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))

			// ensure the virt-v2v pod is cleaned up after a success
			podLabel := "vmimport.v2v.kubevirt.io/vmi-name=" + retrieved.Name
			podList, err := f.K8sClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: podLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(podList.Items)).To(BeZero())
		})

		It("should create started VM", func() {
			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, true)

			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			validCondition := conditions.FindConditionOfType(retrieved.Status.Conditions, v2vv1.Valid)
			Expect(validCondition.Status).To(BeEquivalentTo(corev1.ConditionTrue))
			Expect(*validCondition.Reason).To(BeEquivalentTo(v2vv1.ValidationCompleted))

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			vm := test.validateTargetConfiguration(vmBlueprint.Name)
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))

			// ensure the virt-v2v pod is cleaned up after a success
			podLabel := "vmimport.v2v.kubevirt.io/vmi-name=" + retrieved.Name
			podList, err := f.K8sClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: podLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(podList.Items)).To(BeZero())
		})
	})

	Context(" with in-CR resource mapping", func() {
		table.DescribeTable("should create running VM", func(mappings v2vv1.VmwareMappings, storageClass string) {

			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Vmware.Mappings = &mappings

			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			vm := test.validateTargetConfiguration(vmBlueprint.Name)
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveStorageClass(storageClass, f))
		},
			table.Entry(" for disk", v2vv1.VmwareMappings{
				DiskMappings: &[]v2vv1.StorageResourceMappingItem{
					{Source: v2vv1.Source{Name: &vmware.VM66DiskName}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
				},
			}, f.DefaultStorageClass),
			table.Entry(" for storage domain by name", v2vv1.VmwareMappings{
				StorageMappings: &[]v2vv1.StorageResourceMappingItem{
					{Source: v2vv1.Source{Name: &vmware.VM66DatastoreName}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
				},
			}, f.DefaultStorageClass),
			table.Entry(" for storage domain by id", v2vv1.VmwareMappings{
				StorageMappings: &[]v2vv1.StorageResourceMappingItem{
					{Source: v2vv1.Source{ID: &vmware.VM66Datastore}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
				},
			}, f.DefaultStorageClass))
	})

	Context("when it's successful and the import CR is removed", func() {
		It("should not affect imported VM or VMI", func() {
			By("Creating Virtual Machine Import")
			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, true)
			vmImports := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace)

			created, err := vmImports.Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := vmImports.Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			By("Making sure the imported VM is running")
			Expect(vmBlueprint).To(BeRunning(f))

			When("VM import CR is deleted", func() {
				foreground := metav1.DeletePropagationForeground
				deleteOptions := metav1.DeleteOptions{
					PropagationPolicy: &foreground,
				}
				err = vmImports.Delete(context.TODO(), retrieved.Name, deleteOptions)
				if err != nil {
					Fail(err.Error())
				}

				By("Waiting for VM import removal")
				err = f.EnsureVMImportDoesNotExist(retrieved.Name)
				Expect(err).ToNot(HaveOccurred())
			})


			vmInstanceName := types.NamespacedName{Namespace: namespace, Name: vmBlueprint.Name}
			vmInstance := &v1.VirtualMachineInstance{}
			Consistently(func() (*v1.VirtualMachineInstance, error) {
				err = f.Client.Get(context.TODO(), vmInstanceName, vmInstance)
				if err != nil {
					return nil, err
				}
				return vmInstance, nil

			}, 2*time.Minute, 15*time.Second).ShouldNot(BeNil())
			err = f.Client.Get(context.TODO(), vmInstanceName, vmInstance)
			if err != nil {
				Fail(err.Error())
			}
			Expect(vmInstance.IsRunning()).To(BeTrue())

			vm := &v1.VirtualMachine{}
			err = f.Client.Get(context.TODO(), vmInstanceName, vm)
			if err != nil {
				Fail(err.Error())
			}
		})
	})
})

func (t *basicVmImportTest) validateTargetConfiguration(vmName string) *v1.VirtualMachine {
	vmNamespacedName := types.NamespacedName{Name: vmName, Namespace: t.framework.Namespace.Name}

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
	Expect(cpu.Threads).To(BeEquivalentTo(0))

	By("having no networks")
	Expect(spec.Networks).To(HaveLen(0))

	By("having correct clock settings")
	Expect(spec.Domain.Clock.UTC).ToNot(BeNil())

	By("having correct disk setup")
	disks := spec.Domain.Devices.Disks
	Expect(disks).To(HaveLen(1))
	disk0 := disks[0]
	Expect(disk0.Disk.Bus).To(BeEquivalentTo("virtio"))
	Expect(disk0.Name).To(BeEquivalentTo("dv-f7c371d6-2003-5a48-9859-3bc9a8b08908-204"))

	By("having correct volumes")
	Expect(spec.Volumes).To(HaveLen(1))

	inputDevices := spec.Domain.Devices.Inputs
	tablet := utils.FindTablet(inputDevices)
	Expect(tablet).NotTo(BeNil())
	Expect(tablet.Bus).To(BeEquivalentTo("virtio"))
	Expect(tablet.Type).To(BeEquivalentTo("tablet"))

	return vm
}
