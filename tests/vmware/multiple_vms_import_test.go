package vmware_test

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
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
		f            = fwk.NewFrameworkOrDie("multiple-vms-import", fwk.ProviderVmware)
		namespace    string
		test         = multipleVmsImportTest{framework: f}
		emptyMapping = []v2vv1.NetworkResourceMappingItem{}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		test.namespace = namespace

		secret, err := f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}

		test.secret = secret
	})

	Context("executed in sequence", func() {
		It("should create two started VMs in the same namespace from two different source VMs", func() {
			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmware.VM63, namespace, "vm-no-1", test.secret.Name, &emptyMapping)

			By("Importing and starting second VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmware.VM66, namespace, "vm-no-2", test.secret.Name, &emptyMapping)

			By("Confirm the first VM instance is still running")
			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-no-1", Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))
		})

		It("should fail importing the same source VM with NIC to the same namespace", func() {
			By("Importing and starting first VM")
			networkType := "pod"
			networkMapping := &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &networkType},
			}
			test.importVMWithSecretAndMakeSureItsRunning(vmware.VM70, namespace, "vm-no-1", test.secret.Name, networkMapping)

			By("Importing second VM")
			created, err := test.triggerVMImport(vmware.VM70, namespace, "vm-no-2", test.secret.Name, networkMapping)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeUnsuccessful(f, ""))
		})

		It("should fail importing the same source VM with NIC to different namespace", func() {
			namespace2, err := f.CreateNamespace(f.NsPrefix, make(map[string]string))
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(namespace2)
			secret, err := f.CreateVmwareSecretInNamespace(namespace2.Name)
			if err != nil {
				Fail("Cannot create secret: " + err.Error())
			}

			By("VMs having same MAC address")
			vmID := vmware.VM70
			networkType := "pod"
			networkMapping := &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &networkType},
			}

			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmID, namespace, "vm-no-1", test.secret.Name, networkMapping)

			By("Importing second VM")
			created, err := test.triggerVMImport(vmID, namespace2.Name, "vm-no-2", secret.Name, networkMapping)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeUnsuccessful(f, ""))
		})

		It("should create one started VM from two imports of the same NIC-less source VM with same target name to one namespace ", func() {
			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmware.VM63, namespace, "vm-no-1", test.secret.Name, &emptyMapping)

			By("Importing second VM")
			created, err := test.triggerVMImport(vmware.VM63, namespace, "vm-no-1", test.secret.Name, &emptyMapping)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			By("Having only one VM imported in the end")
			vms, err := f.KubeVirtClient.VirtualMachine(namespace).List(&metav1.ListOptions{})
			if err != nil {
				Fail(err.Error())
			}
			Expect(vms.Items).To(HaveLen(1))
			Expect(vms.Items[0].Name).To(BeEquivalentTo("vm-no-1"))
		})

		It("should create two started VMs from the same NIC-less source VM and with same target name in different namespaces", func() {
			namespace2, err := f.CreateNamespace(f.NsPrefix, make(map[string]string))
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(namespace2)
			secret, err := f.CreateVmwareSecretInNamespace(namespace2.Name)
			if err != nil {
				Fail("Cannot create secret: " + err.Error())
			}

			By("Importing and starting first VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmware.VM63, namespace, "vm-no-1", test.secret.Name, &emptyMapping)

			By("Importing and starting second VM")
			test.importVMWithSecretAndMakeSureItsRunning(vmware.VM63, namespace2.Name, "vm-no-1", secret.Name, &emptyMapping)

			By("Confirm the first VM instance is still running")
			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-no-1", Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))
		})
	})

	Context("executed in parallel", func() {
		It("should create two started VMs in the same namespace from two different source VMs", func() {
			By("Triggering the first VM import")
			createdVM1, err := test.triggerVMImport(vmware.VM63, namespace, "vm-no-1", test.secret.Name, &emptyMapping)
			Expect(err).ToNot(HaveOccurred())

			By("Triggering the second VM import")
			createdVM2, err := test.triggerVMImport(vmware.VM66, namespace, "vm-no-2", test.secret.Name, &emptyMapping)
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

func (t *multipleVmsImportTest) importVMWithSecretAndMakeSureItsRunning(vmID string, namespace string, vmName string, secretName string, networkMappings *[]v2vv1.NetworkResourceMappingItem) {
	created, err := t.triggerVMImport(vmID, namespace, vmName, secretName, networkMappings)
	Expect(created).To(BeSuccessful(t.framework))

	retrieved, _ := t.framework.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	effectiveTargetVMName := retrieved.Status.TargetVMName
	Expect(effectiveTargetVMName).To(BeEquivalentTo(vmName))

	vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: effectiveTargetVMName, Namespace: namespace}}
	Expect(vmBlueprint).To(BeRunning(t.framework))
}

func (t *multipleVmsImportTest) triggerVMImport(vmID string, namespace string, vmName string, secretName string, networkMappings *[]v2vv1.NetworkResourceMappingItem) (*v2vv1.VirtualMachineImport, error) {
	vmi := utils.VirtualMachineImportCrWithName(fwk.ProviderVmware, vmID, namespace, secretName, t.framework.NsPrefix+"-"+vmID, true, vmName)
	vmi.Spec.Source.Vmware.Mappings = &v2vv1.VmwareMappings{NetworkMappings: networkMappings}
	created, err := t.framework.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

	Expect(err).NotTo(HaveOccurred())
	return created, err
}
