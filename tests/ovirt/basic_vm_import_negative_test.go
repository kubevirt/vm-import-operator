package ovirt_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/ovirt"
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
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

	It("should fail for missing secret", func() {
		vmID := vms.MissingOVirtSecretVmId
		vmi := utils.VirtualMachineImportCr(vmID, namespace, "no-such-secret", f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1alpha1.SecretNotFound)))
	})

	It("should fail for invalid secret", func() {
		vmID := vms.InvalidOVirtSecretVmId
		invalidSecret := test.createInvalidSecret()
		vmi := utils.VirtualMachineImportCr(vmID, namespace, invalidSecret.Name, f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1alpha1.UninitializedProvider)))
	})

	table.DescribeTable("should fail for invalid ", func(vmID string, apiURL string, username string, password string, caCert string) {
		invalidSecret, err := f.CreateOvirtSecret(apiURL, username, password, caCert)
		if err != nil {
			Fail(err.Error())
		}
		vmi := utils.VirtualMachineImportCr(vmID, namespace, invalidSecret.Name, f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1alpha1.SourceVMNotFound)))
	},
		table.Entry("oVirt URL", vms.InvalidOVirtUrlVmID, "", ovirt.Username, ovirt.Password, ovirt.CACert),
		table.Entry("oVirt username", vms.InvalidOVirtUsernameVmID, ovirt.ApiURL, "", ovirt.Password, ovirt.CACert),
		table.Entry("oVirt password", vms.InvalidOVirtPasswordVmID, ovirt.ApiURL, ovirt.Username, "", ovirt.CACert),
		table.Entry("oVirt CA cert", vms.InvalidOVirtCACertVmID, ovirt.ApiURL, ovirt.Username, ovirt.Password, "garbage"),
	)

	It("should fail for non-existing VM ID", func() {
		vmID := "does-not-exist"
		vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)

		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1alpha1.SourceVMNotFound)))
	})

	It("should fail for missing specified external mapping", func() {
		vmID := vms.MissingExternalResourceMappingVmID
		vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{Name: "does-not-exist", Namespace: &namespace}

		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1alpha1.ResourceMappingNotFound)))
	})
})

func (t *basicVmImportNegativeTest) createInvalidSecret() *corev1.Secret {
	f := t.framework
	namespace := f.Namespace.Name
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.NsPrefix,
			Namespace:    namespace,
		},
		StringData: map[string]string{"ovirt": "garbage"},
	}
	created, err := f.K8sClient.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		Fail(err.Error())
	}
	return created
}
func (t *basicVmImportNegativeTest) prepareInvalidVm(vmID string, diskSize string) {
	diskXML := t.framework.LoadTemplate("disks/invalid-disk.xml", map[string]string{"@DISKSIZE": diskSize})
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/invalid-disk.xml")
	t.prepareVmWithDiskXML(vmID, vmID, diskXML, diskAttachmentsXML)
}

func (t *basicVmImportNegativeTest) prepareVm(vmID string) {
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	t.prepareVmWithDiskXML(vmID, "disk-1", diskXML, diskAttachmentsXML)
}

func (t *basicVmImportNegativeTest) prepareVmWithDiskXML(vmID string, diskID string, diskXML string, diskAttachmentsXML string) {
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	vmXML := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	nicsXML := t.framework.LoadFile("nics/empty.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/disks/"+diskID, &diskXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
