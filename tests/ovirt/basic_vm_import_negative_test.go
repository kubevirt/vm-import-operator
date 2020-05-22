package ovirt_test

import (
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	v1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type basicVmImportNegativeTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM import", func() {

	var (
		f         = fwk.NewFrameworkOrDie("basic-vm-import-negative")
		secret    corev1.Secret
		namespace string
		test      = basicVmImportNegativeTest{f}
		err       error
	)

	BeforeEach(func() {
		secret, err = f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		namespace = f.Namespace.Name
	})

	table.DescribeTable("should fail import with  ", func(diskSize string) {
		vmID := vms.InvalidDiskID
		vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
		test.prepareInvalidVm(vmID, diskSize)

		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		// Check if vm import failed:
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveDataVolumeCreationFailure(f))

		retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Check if virtual machine was removed after failure:
		vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
		_, err = f.KubeVirtClient.VirtualMachine(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
		Expect(errors.IsNotFound(err)).To(BeTrue())

		// Check if data volumes was removed after failure:
		for _, createdDv := range retrieved.Status.DataVolumes {
			dv := cdiv1.DataVolume{ObjectMeta: metav1.ObjectMeta{Name: createdDv.Name, Namespace: namespace}}
			_, err = f.CdiClient.CdiV1alpha1().DataVolumes(namespace).Get(dv.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		}
	},
		table.Entry("invalid disk image", "4096"),
		table.Entry("invalid disk size", "0"),
	)
})

func (t *basicVmImportNegativeTest) prepareInvalidVm(vmID string, diskSize string) {
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/invalid-disk.xml")
	diskXml := t.framework.LoadTemplate("disks/invalid-disk.xml", map[string]string{"@DISKSIZE": diskSize})
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	vmXml := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/disks/"+vmID, &diskXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
