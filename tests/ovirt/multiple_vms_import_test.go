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

type multipleVmsImportTest struct {
	framework *fwk.Framework
	namespace string
	secret    corev1.Secret
}

var _ = Describe("Multiple VMs import ", func() {
	var (
		f         = fwk.NewFrameworkOrDie("multiple-vms-import")
		namespace string
		test      = multipleVmsImportTest{framework: f}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		test.namespace = namespace
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		test.secret = s
	})

	Context("executed in sequence", func() {
		It("should create two started VMs in the same namespace from two different source VMs", func() {
			By("Importing and starting first VM")
			test.stubAndImportVMAndMakeSureItsRunning(vms.MultipleVmsNo1VmID, namespace, "vm-no-1", "56:6f:05:0f:00:05")

			By("Importing and starting second VM")
			test.stubAndImportVMAndMakeSureItsRunning(vms.MultipleVmsNo2VmID, namespace, "vm-no-2", "56:6f:05:0f:00:06")

			By("Confirm the first VM instance is still running")
			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-no-1", Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))
		})

		It("should fail importing the same source VM with NIC to the same namespace", func() {
			vmID := vms.MultipleVmsNo1VmID
			By("VMs having same MAC address")
			test.stub(vmID, "56:6f:05:0f:00:05")

			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmID, namespace, "vm-no-1", test.secret.Name)

			By("Importing second VM")
			created, err := test.triggerVMImport(vmID, namespace, "vm-no-2", test.secret.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeUnsuccessful(f, ""))
		})

		It("should fail importing the same source VM with NIC to different namespace", func() {
			namespace2, err := f.CreateNamespace(f.NsPrefix, make(map[string]string))
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(namespace2)
			secret, err := f.CreateOvirtSecretInNamespaceFromBlueprint(namespace2.Name)
			if err != nil {
				Fail("Cannot create secret: " + err.Error())
			}

			By("VMs having same MAC address")
			vmID := vms.MultipleVmsNo1VmID
			test.stub(vmID, "56:6f:05:0f:00:05")

			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmID, namespace, "vm-no-1", test.secret.Name)

			By("Importing second VM")
			created, err := test.triggerVMImport(vmID, namespace2.Name, "vm-no-2", secret.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeUnsuccessful(f, ""))
		})

		It("should create one started VM from two imports of the same NIC-less source VM with same target name to one namespace ", func() {
			vmID := vms.MultipleVmsNo1VmID
			vmName := "vm-no-1"
			nicsXML := f.LoadFile("nics/empty.xml")
			test.stubWithNicsXML(vmID, nicsXML)

			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmID, namespace, vmName, test.secret.Name)

			By("Importing second VM")
			created, err := test.triggerVMImport(vmID, namespace, vmName, test.secret.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			By("Having only one VM imported in the end")
			vms, err := f.KubeVirtClient.VirtualMachine(namespace).List(&metav1.ListOptions{})
			if err != nil {
				Fail(err.Error())
			}
			Expect(vms.Items).To(HaveLen(1))
			Expect(vms.Items[0].Name).To(BeEquivalentTo(vmName))
		})

		It("should create two started VMs from the same NIC-less source VM and with same target name in different namespaces", func() {
			namespace2, err := f.CreateNamespace(f.NsPrefix, make(map[string]string))
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(namespace2)
			secret, err := f.CreateOvirtSecretInNamespaceFromBlueprint(namespace2.Name)
			if err != nil {
				Fail("Cannot create secret: " + err.Error())
			}
			vmID := vms.MultipleVmsNo1VmID
			nicsXML := f.LoadFile("nics/empty.xml")
			test.stubWithNicsXML(vmID, nicsXML)

			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmID, namespace, "vm-no-1", test.secret.Name)

			By("Importing and starting second VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmID, namespace2.Name, "vm-no-1", secret.Name)

			By("Confirm the first VM instance is still running")
			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-no-1", Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))
		})
	})

	Context("executed in parallel", func() {
		It("should create two started VMs in the same namespace from two different source VMs", func() {
			vmID1 := vms.MultipleVmsNo1VmID
			test.stub(vmID1, "56:6f:05:0f:00:06")

			vmID2 := vms.MultipleVmsNo2VmID
			nicsXML := f.LoadTemplate("nics/mac-template.xml", map[string]string{"@MAC": "56:6f:05:0f:00:05"})
			disk2ID := "cirros2"
			diskAttachmentsXML := f.LoadTemplate("disk-attachments/disk_id-template.xml", map[string]string{"@DISKID": disk2ID})
			diskXML := f.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344", "@DISKID": disk2ID})
			test.stubWithDiskAndNicsXML(vmID2, nicsXML, diskAttachmentsXML, diskXML, disk2ID)

			By("Triggering the first VM import")
			createdVM1, err := test.triggerVMImport(vmID1, namespace, "vm-no-1", test.secret.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Triggering the second VM import")
			createdVM2, err := test.triggerVMImport(vmID2, namespace, "vm-no-2", test.secret.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Both import being successful")
			Expect(createdVM1).To(BeSuccessful(f))
			Expect(createdVM2).To(BeSuccessful(f))

			By("Both VM instances running")
			Expect(v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-no-1", Namespace: namespace}}).To(BeRunning(f))
			Expect(v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-no-2", Namespace: namespace}}).To(BeRunning(f))
		})
	})
})

func (t *multipleVmsImportTest) stubAndImportVMAndMakeSureItsRunning(vmID string, namespace string, vmName string, mac string) {
	t.stub(vmID, mac)
	t.importVMWithSecretAndMakeSureItsRunning(vmID, namespace, vmName, t.secret.Name)
}

func (t *multipleVmsImportTest) importVMWithSecretAndMakeSureItsRunning(vmID string, namespace string, vmName string, secretName string) {
	created, err := t.triggerVMImport(vmID, namespace, vmName, secretName)
	Expect(created).To(BeSuccessful(t.framework))

	retrieved, _ := t.framework.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	effectiveTargetVMName := retrieved.Status.TargetVMName
	Expect(effectiveTargetVMName).To(BeEquivalentTo(vmName))

	vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: effectiveTargetVMName, Namespace: namespace}}
	Expect(vmBlueprint).To(BeRunning(t.framework))
}

func (t *multipleVmsImportTest) triggerVMImport(vmID string, namespace string, vmName string, secretName string) (*v2vv1alpha1.VirtualMachineImport, error) {
	vmi := utils.VirtualMachineImportCrWithName(vmID, namespace, secretName, t.framework.NsPrefix+"-"+vmID, true, vmName)
	vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
		NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
			{Source: v2vv1alpha1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType},
		},
	}
	created, err := t.framework.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

	Expect(err).NotTo(HaveOccurred())
	return created, err
}

func (t *multipleVmsImportTest) stub(vmID string, mac string) {
	nicsXML := t.framework.LoadTemplate("nics/mac-template.xml", map[string]string{"@MAC": mac})
	t.stubWithNicsXML(vmID, nicsXML)
}

func (t *multipleVmsImportTest) stubWithNicsXML(vmID string, nicsXML string) {
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	t.stubWithDiskAndNicsXML(vmID, nicsXML, diskAttachmentsXML, diskXML, "disk-1")
}

func (t *multipleVmsImportTest) stubWithDiskAndNicsXML(vmID string, nicsXML string, diskAttachmentsXML string, diskXML string, diskID string) {
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXML := t.framework.LoadFile("graphic-consoles/vnc.xml")
	networkXML := t.framework.LoadFile("networks/net-1.xml")
	vnicProfileXML := t.framework.LoadFile("vnic-profiles/vnic-profile-1.xml")
	vmXML := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/disks/"+diskID, &diskXML).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXML).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", &vnicProfileXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
