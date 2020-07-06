package ovirt_test

import (
	"strings"

	"github.com/onsi/ginkgo/extensions/table"

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
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		test.secret = s
	})

	AfterEach(func() {
		f.CleanUp()
		// Make sure we clean up the config map as the last of, otherwise it's not
		// possible to remove the vm with livestrategy enabled.
		cleanUpConfigMap(f)
	})

	It("placement policy: 'migratable' and LiveMigration enabled", func() {
		vmID := vms.PlacementPolicyAffinityVmIDPrefix + "migratable"
		configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get("kubevirt-config", metav1.GetOptions{})
		if err != nil {
			Fail(err.Error())
		}
		configMap.Data["feature-gates"] = configMap.Data["feature-gates"] + ",LiveMigration"
		f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Update(configMap)
		test.stub(vmID, "placement-policy-affinity-template.xml", map[string]string{"@AFFINITY": "migratable"})
		test.ensureVMIsRunningOnStorage(vmID, &[]v2vv1alpha1.ResourceMappingItem{
			{Source: v2vv1alpha1.Source{ID: &vms.StorageDomainID}, Target: v2vv1alpha1.ObjectIdentifier{Name: f.NfsStorageClass}},
		})
	})
})

var _ = Describe("Import", func() {

	var (
		f    = fwk.NewFrameworkOrDie("various-vm-configurations")
		test = variousVMConfigurationsTest{framework: f}
	)

	BeforeEach(func() {
		test.namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		test.secret = s
	})

	Context("should create started VM configured with", func() {
		table.DescribeTable("UTC-compatible timezone", func(timezone string) {
			vmID := vms.UtcCompatibleTimeZoneVmID
			test.stub(vmID, "timezone-template.xml", map[string]string{"@TIMEZONE": timezone})
			vm := test.ensureVMIsRunning(vmID)

			spec := vm.Spec.Template.Spec
			Expect(spec.Domain.Clock.UTC).ToNot(BeNil())
		},
			table.Entry("TzData-compatible: `Africa/Abidjan`", "Africa/Abidjan"),
			table.Entry("Windows-specific: `GMT Standard Time`", "GMT Standard Time"),
		)

		table.DescribeTable("BIOS type", func(inBIOSType string, targetBootloader v1.Bootloader) {
			vmID := vms.BIOSTypeVmIDPrefix + inBIOSType
			test.stub(vmID, "bios-type-template.xml", map[string]string{"@BIOSTYPE": inBIOSType})
			vm := test.ensureVMIsRunning(vmID)
			spec := vm.Spec.Template.Spec
			Expect(*spec.Domain.Firmware.Bootloader).To(BeEquivalentTo(targetBootloader))
		},
			table.Entry("q35_sea_bios", "q35_sea_bios", v1.Bootloader{BIOS: &v1.BIOS{}}),
			table.Entry("q35_secure_boot", "q35_secure_boot", v1.Bootloader{BIOS: &v1.BIOS{}}),
			table.Entry("q35_ovmf", "q35_ovmf", v1.Bootloader{EFI: &v1.EFI{}}))

		table.DescribeTable("architecture", func(inArch string, targetArch string) {
			vmID := vms.ArchitectureVmIDPrefix + inArch
			test.stub(vmID, "architecture-template.xml", map[string]string{"@ARCH": inArch})
			vm := test.ensureVMIsRunning(vmID)
			spec := vm.Spec.Template.Spec
			Expect(spec.Domain.Machine.Type).To(BeEquivalentTo(targetArch))
		},
			table.Entry("undefined", "undefined", "q35"))

		It("i6300esb watchdog", func() {
			vmID := vms.I6300esbWatchdogVmID
			wdXML := test.framework.LoadTemplate("watchdogs/model-template.xml", map[string]string{"@MODEL": "i6300esb"})
			stubbing := sapi.NewStubbingBuilder().StubGet("/ovirt-engine/api/vms/"+vmID+"/watchdogs", &wdXML).Build()
			err := f.OvirtStubbingClient.Stub(stubbing)
			if err != nil {
				Fail(err.Error())
			}
			test.stub(vmID, "watchdog-vm.xml", map[string]string{})

			test.ensureVMIsRunning(vmID)
		})

		It("exact CPU pinning", func() {
			err := f.AddLabelToAllNodes("cpumanager", "true")
			if err != nil {
				Fail(err.Error())
			}
			defer f.RemoveLabelFromNodes("cpumanager")
			vmID := vms.CPUPinningVmID
			test.stub(vmID, "cpu-pinning-template.xml", map[string]string{})
			vm := test.ensureVMIsRunning(vmID)
			spec := vm.Spec.Template.Spec
			Expect(spec.Domain.CPU.DedicatedCPUPlacement).To(BeTrue())
		})
	})
	table.DescribeTable("should create started VM configured with", func(vmID string, templateFile string, macros map[string]string) {
		test.stub(vmID, templateFile, macros)
		test.ensureVMIsRunning(vmID)
	},
		table.Entry("ovirt origin", vms.OvirtOriginVmID, "origin-template.xml", map[string]string{"@ORIGIN": "ovirt"}),
		table.Entry("placement policy affinity: 'user_migratable'", vms.PlacementPolicyAffinityVmIDPrefix+"user_migratable", "placement-policy-affinity-template.xml", map[string]string{"@AFFINITY": "user_migratable"}),
		table.Entry("placement policy affinity: 'pinned'", vms.PlacementPolicyAffinityVmIDPrefix+"pinned", "placement-policy-affinity-template.xml", map[string]string{"@AFFINITY": "pinned"}),
		table.Entry("disabled USB", vms.UsbDisabledVmID, "usb-template.xml", map[string]string{"@ENABLED": "false"}))

	It("should create started VM from VM in 'up' status", func() {
		vmID := vms.UpStatusVmID
		upVMXML := f.LoadTemplate("vms/status-template.xml", map[string]string{"@VMID": vmID, "@VMSTATUS": "up"})
		downVMXML := f.LoadTemplate("vms/status-template.xml", map[string]string{"@VMID": vmID, "@VMSTATUS": "down"})
		actionXML := "<action/>"
		builder := test.createVMResourcesStubs(vmID).
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
			Stub(sapi.Stubbing{
				Path:   "/ovirt-engine/api/vms/" + vmID,
				Method: "GET",
				Responses: []sapi.RepeatedResponse{
					// VM is UP at the beginning and shortly after shutdown request was issued
					{
						ResponseBody: &upVMXML,
						ResponseCode: 200,
						// Make the oVirt client wait 3 polling cycles
						Times: 3,
					},
					// After shutdown call eventually oVirt should report VM being down
					{
						ResponseBody: &downVMXML,
						ResponseCode: 200,
					},
				},
			})
		err := f.OvirtStubbingClient.Stub(builder.Build())
		if err != nil {
			Fail(err.Error())
		}

		test.ensureVMIsRunning(vmID)
	})
})

func cleanUpConfigMap(f *fwk.Framework) {
	configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get("kubevirt-config", metav1.GetOptions{})
	if err != nil {
		Fail(err.Error())
	}
	configMap.Data["feature-gates"] = strings.ReplaceAll(configMap.Data["feature-gates"], ",LiveMigration", "")
	_, err = f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Update(configMap)
	if err != nil {
		Fail(err.Error())
	}
}

func (t *variousVMConfigurationsTest) ensureVMIsRunning(vmID string) *v1.VirtualMachine {
	return t.ensureVMIsRunningOnStorage(vmID, nil)
}

func (t *variousVMConfigurationsTest) ensureVMIsRunningOnStorage(vmID string, storageMappings *[]v2vv1alpha1.ResourceMappingItem) *v1.VirtualMachine {
	f := t.framework
	namespace := t.framework.Namespace.Name
	vmi := utils.VirtualMachineImportCr(vmID, namespace, t.secret.Name, f.NsPrefix, true)
	vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
		NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
			{Source: v2vv1alpha1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.PodType},
		},
	}
	if storageMappings != nil {
		vmi.Spec.Source.Ovirt.Mappings.StorageMappings = storageMappings
	}

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
	vmMacros["@VMID"] = vmID
	vmXML := t.framework.LoadTemplate("vms/"+vmFile, vmMacros)
	builder := t.createVMResourcesStubs(vmID).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXML)
	err := t.framework.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}

func (t *variousVMConfigurationsTest) createVMResourcesStubs(vmID string) *sapi.StubbingBuilder {
	domainXML := t.framework.LoadFile("storage-domains/domain-1.xml")
	diskAttachmentsXML := t.framework.LoadFile("disk-attachments/one.xml")
	diskXML := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	consolesXML := t.framework.LoadFile("graphic-consoles/vnc.xml")

	nicsXML := t.framework.LoadFile("nics/one.xml")
	networkXML := t.framework.LoadFile("networks/net-1.xml")
	vnicProfileXML := t.framework.LoadFile("vnic-profiles/vnic-profile-1.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXML).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXML).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXML).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXML).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", &vnicProfileXML).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXML)

	return builder
}
