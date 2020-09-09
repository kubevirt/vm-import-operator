package ovirt_test

import (
	"strings"
	"time"

	ovirtenv "github.com/kubevirt/vm-import-operator/tests/env/ovirt"

	"github.com/kubevirt/vm-import-operator/tests"

	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/onsi/ginkgo/extensions/table"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type basicVMImportNegativeTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM import", func() {

	var (
		f         = fwk.NewFrameworkOrDie("basic-vm-import-negative")
		secret    corev1.Secret
		namespace string
		test      = basicVMImportNegativeTest{f}
		err       error
	)

	BeforeEach(func() {
		secret, err = f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		namespace = f.Namespace.Name
	})

	table.DescribeTable("should fail import with  ", func(diskSize string) {
		vmID := vms.InvalidDiskID
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)
		test.prepareInvalidVm(vmID, diskSize)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		// Check if vm import failed:
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveDataVolumeCreationFailure(f))

		retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
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
		table.Entry("invalid disk size", "1"),
	)

	It("should fail for missing secret", func() {
		vmID := vms.MissingOVirtSecretVmId
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, "no-such-secret", f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.SecretNotFound)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	})

	It("should fail for invalid secret", func() {
		vmID := vms.InvalidOVirtSecretVmId
		invalidSecret := test.createInvalidSecret()
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, invalidSecret.Name, f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.UninitializedProvider)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	})

	table.DescribeTable("should fail for invalid ", func(vmID string, ovirtEnv *ovirtenv.Environment) {
		invalidSecret, err := f.CreateOvirtSecret(*ovirtEnv)
		if err != nil {
			Fail(err.Error())
		}
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, invalidSecret.Name, f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.UninitializedProvider)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	},
		table.Entry("oVirt URL", vms.InvalidOVirtUrlVmID, ovirtenv.NewFakeOvirtEnvironment(f.ImageioInstallNamespace, f.OVirtCA).WithAPIURL("")),
		table.Entry("oVirt username", vms.InvalidOVirtUsernameVmID, ovirtenv.NewFakeOvirtEnvironment(f.ImageioInstallNamespace, f.OVirtCA).WithUsername("")),
		table.Entry("oVirt password", vms.InvalidOVirtPasswordVmID, ovirtenv.NewFakeOvirtEnvironment(f.ImageioInstallNamespace, f.OVirtCA).WithPassword("")),
		table.Entry("oVirt CA cert", vms.InvalidOVirtCACertVmID, ovirtenv.NewFakeOvirtEnvironment(f.ImageioInstallNamespace, "")),
	)

	It("should fail for invalid CA Cert", func() {
		vmID := vms.InvalidOVirtCACertVmID
		ovirtEnv := ovirtenv.NewFakeOvirtEnvironment(f.ImageioInstallNamespace, "garbage")
		invalidSecret, err := f.CreateOvirtSecret(*ovirtEnv)
		if err != nil {
			Fail(err.Error())
		}
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, invalidSecret.Name, f.NsPrefix, true)
		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.UnreachableProvider)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	})

	It("should fail for non-existing VM ID", func() {
		vmID := "does-not-exist"
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.SourceVMNotFound)))
	})

	It("should fail for missing specified external mapping", func() {
		vmID := vms.MissingExternalResourceMappingVmID
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{Name: "does-not-exist", Namespace: &namespace}

		test.prepareVm(vmID)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.ResourceMappingNotFound)))
	})

	It("should be stuck when source VM does not shut down", func() {
		vmID := vms.BasicVmID
		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)

		vmXML := f.LoadTemplate("vms/status-template.xml", map[string]string{"@VMID": vmID, "@VMSTATUS": "up"})
		actionXML := "<action/>"
		builder := test.prepareVmResourcesStub(vmID).
			Stub(sapi.Stubbing{
				Path:   "/ovirt-engine/api/vms/" + vmID + "/shutdown",
				Method: "POST",
				Responses: []sapi.RepeatedResponse{
					{
						ResponseBody: &actionXML,
						ResponseCode: 200,
					},
				},
			}).
			StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
		test.recordStubbing(builder)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())

		Consistently(func() (*v2vv1.VirtualMachineImportCondition, error) {
			retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(f.Namespace.Name).Get(created.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			condition := conditions.FindConditionOfType(retrieved.Status.Conditions, v2vv1.Processing)
			return condition, nil

		}, 5*time.Minute, time.Minute).Should(BeNil())
	})

	It("should fail when ImportWithoutTemplate feature gate is disabled and VM template can't be found", func() {
		vmID := vms.BasicVmID
		configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get("kubevirt-config", metav1.GetOptions{})
		if err != nil {
			Fail(err.Error())
		}
		configMap.Data["feature-gates"] = strings.ReplaceAll(configMap.Data["feature-gates"], ",ImportWithoutTemplate", "")

		f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Update(configMap)
		defer test.cleanUpConfigMap()

		vmi := utils.VirtualMachineImportCr(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Ovirt.Mappings = &v2vv1.OvirtMappings{
			NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType},
			},
		}
		test.prepareVm(vmID)
		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveTemplateMatchingFailure(f))
	})

	It("should fail when targetVMName is too long", func() {
		vmID := vms.BasicVmID
		vmName := strings.Repeat("x", 64)
		vmi := utils.VirtualMachineImportCrWithName(fwk.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true, vmName)
		test.prepareVm(vmID)

		_, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.targetVmName in body should be at most 63 chars long"))
	})
})

func (t *basicVMImportNegativeTest) createInvalidSecret() *corev1.Secret {
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
func (t *basicVMImportNegativeTest) prepareInvalidVm(vmID string, diskSize string) {
	diskXML := t.framework.LoadTemplate("disks/disk_id-template.xml", map[string]string{"@DISKSIZE": diskSize, "@DISKID": "invalid"})
	diskAttachmentsXML := t.framework.LoadTemplate("disk-attachments/disk_id-template.xml", map[string]string{"@DISKID": "invalid"})
	t.prepareVmWithDiskXML(vmID, vmID, diskXML, diskAttachmentsXML)
}

func (t *basicVMImportNegativeTest) prepareVm(vmID string) {
	vmXML := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	builder := t.prepareVmResourcesStub(vmID).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	t.recordStubbing(builder)
}

func (t *basicVMImportNegativeTest) prepareVmWithDiskXML(vmID string, diskID string, diskXML string, diskAttachmentsXML string) {
	vmXML := t.framework.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	builder := t.prepareVMResourceStubWithDiskData(vmID, diskID, diskXML, diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	t.recordStubbing(builder)
}

func (t *basicVMImportNegativeTest) prepareVmResourcesStub(vmID string) *sapi.StubbingBuilder {
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	return t.prepareVMResourceStubWithDiskData(vmID, "disk-1", diskXML, diskAttachmentsXML)
}

func (t *basicVMImportNegativeTest) prepareVMResourceStubWithDiskData(vmID string, diskID string, diskXML string, diskAttachmentsXML string) *sapi.StubbingBuilder {
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	nicsXML := t.framework.LoadFile("nics/empty.xml")
	return sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/disks/"+diskID, &diskXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML)
}

func (t *basicVMImportNegativeTest) recordStubbing(builder *sapi.StubbingBuilder) {
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}

func (t *basicVMImportNegativeTest) cleanUpConfigMap() {
	f := t.framework
	configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get("kubevirt-config", metav1.GetOptions{})
	if err != nil {
		Fail(err.Error())
	}
	configMap.Data["feature-gates"] = configMap.Data["feature-gates"] + ",ImportWithoutTemplate"
	_, err = f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Update(configMap)
	if err != nil {
		Fail(err.Error())
	}
}
