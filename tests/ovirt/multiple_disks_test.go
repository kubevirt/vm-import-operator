package ovirt_test

import (
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

type multipleDisksTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM import ", func() {
	var (
		f         = fwk.NewFrameworkOrDie("multiple-disks")
		secret    corev1.Secret
		namespace string
		test      = multipleDisksTest{f}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	Context("for VM with two disks", func() {
		It("should create started VM", func() {
			vmID := vms.TwoDisksVmID
			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, tests.TrueVar)
			vmi.Spec.StartVM = &tests.TrueVar
			test.stub(vmID)
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

func (t *multipleDisksTest) stub(vmID string) {
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/two.xml")
	disk1Xml := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	disk2Xml := t.framework.LoadTemplate("disks/disk-2.xml", map[string]string{"@DISKSIZE": "46137344"})
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXml := t.framework.LoadFile("graphic-consoles/empty.xml")
	vmXml := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	nicsXml := t.framework.LoadFile("nics/empty.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
		StubGet("/ovirt-engine/api/disks/disk-1", &disk1Xml).
		StubGet("/ovirt-engine/api/disks/disk-2", &disk2Xml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
