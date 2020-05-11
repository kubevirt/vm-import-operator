package tests_test

import (
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("VM import ", func() {
	var (
		f         = fwk.NewFrameworkOrDie("multiple-disks")
		secret    corev1.Secret
		namespace string
	)
	var (
		trueVar = true
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	Context("for VM with two disks", func() {
		It("should create started VM", func() {
			vmi := utils.VirtualMachineImportCr("two-disks", namespace, secret.Name, f.NsPrefix, trueVar)
			vmi.Spec.StartVM = &trueVar

			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			vm, err := f.KubeVirtClient.VirtualMachine(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			if err != nil {
				Fail(err.Error())
			}
			spec := vm.Spec.Template.Spec

			By("having correct disk setup")
			disks := spec.Domain.Devices.Disks
			Expect(disks).To(HaveLen(2))

			disk1 := disks[0]
			disk2 := disks[1]
			if disk1.BootOrder == nil {
				disk2, disk1 = disk1, disk2
			}

			Expect(disk1.Disk.Bus).To(BeEquivalentTo("virtio"))
			Expect(*disk1.BootOrder).To(BeEquivalentTo(1))

			Expect(disk2.Disk.Bus).To(BeEquivalentTo("sata"))
			Expect(disk2.BootOrder).To(BeNil())

			By("having correct volumes")
			Expect(spec.Volumes).To(HaveLen(2))

			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))
			Expect(vm.Spec.Template.Spec.Volumes[1].DataVolume.Name).To(HaveDefaultStorageClass(f))
		})
	})
})
