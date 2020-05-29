package ovirt_test

import (
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

type variousVMConfigurationsTest struct {
	framework *fwk.Framework
	secret    corev1.Secret
	namespace string
}

var _ = Describe("Import", func() {

	var (
		f    = fwk.NewFrameworkOrDie("various-vm-configurations")
		test = variousVMConfigurationsTest{framework: f}
	)

	BeforeEach(func() {
		test.namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		test.secret = s
	})

	Context("should create started VM configured with", func() {
		It("UTC-compatible timezone", func() {
			vmID := vms.UtcCompatibleTimeZone
			test.stub(vmID, "timezone-template.xml", map[string]string{"@TIMEZONE": "Africa/Abidjan"})
			vm := test.ensureVMIsRunning(vmID)

			spec := vm.Spec.Template.Spec
			Expect(spec.Domain.Clock.UTC).ToNot(BeNil())
		})
	})
})

func (t *variousVMConfigurationsTest) ensureVMIsRunning(vmID string) *v1.VirtualMachine {
	f := t.framework
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(vmID, namespace, t.secret.Name, f.NsPrefix, true)
	created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

	Expect(err).NotTo(HaveOccurred())
	Expect(created).To(BeSuccessful(f))

	retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
	Expect(vmBlueprint).To(BeRunning(f))

	vm, _ := f.KubeVirtClient.VirtualMachine(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
	return vm
}

func (t *variousVMConfigurationsTest) stub(vmID string, vmFile string, vmMacros map[string]string) {
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	consolesXML := t.framework.LoadFile("graphic-consoles/vnc.xml")
	vmMacros["@VMID"] = vmID
	vmXML := t.framework.LoadTemplate("vms/"+vmFile, vmMacros)
	nicsXML := t.framework.LoadFile("nics/empty.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
