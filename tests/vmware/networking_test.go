package vmware_test

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/tests"
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

var _ = Describe("Import of VM ", func() {
	var (
		f           = fwk.NewFrameworkOrDie("networking", fwk.ProviderVmware)
		secret      corev1.Secret
		namespace   string
		networkName string
		err         error
	)

	BeforeSuite(func() {
		err := f.ConfigureNodeNetwork()
		if err != nil {
			Fail(err.Error())
		}
	})

	BeforeEach(func() {
		namespace = f.Namespace.Name
		secret, err = f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		nad, err := f.CreateLinuxBridgeNetworkAttachmentDefinition()
		if err != nil {
			Fail(err.Error())
		}
		networkName = nad.Name

	})

	Context("with multus network", func() {
		It("should create running VM", func() {
			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Vmware.Mappings = &v2vv1.VmwareMappings{
				NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
					{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &tests.MultusType, Target: v2vv1.ObjectIdentifier{
						Name:      networkName,
						Namespace: &f.Namespace.Name,
					}},
				},
			}
			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("having correct network configuration")
			vm, _ := f.KubeVirtClient.VirtualMachineInstance(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			spec := vm.Spec
			Expect(spec.Networks).To(HaveLen(1))
			Expect(spec.Networks[0].Multus).ToNot(BeNil())
			Expect(spec.Networks[0].Multus.NetworkName).To(BeEquivalentTo(networkName))

			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(1))
			nic := spec.Domain.Devices.Interfaces[0]
			Expect(nic.Name).To(BeEquivalentTo(spec.Networks[0].Name))
			Expect(nic.MacAddress).To(BeEquivalentTo(vmware.VM70MacAddress))
			Expect(nic.Bridge).ToNot(BeNil())
			Expect(nic.Model).To(BeEquivalentTo("virtio"))

			Expect(vm.Status.Interfaces).To(HaveLen(1))
			Expect(vm.Status.Interfaces[0].Name).To(BeEquivalentTo(spec.Networks[0].Name))
			Expect(vm.Status.Interfaces[0].MAC).To(BeEquivalentTo(vmware.VM70MacAddress))
		})
	})
})
