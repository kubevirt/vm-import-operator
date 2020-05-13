package tests_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	vms "github.com/kubevirt/vm-import-operator/tests/ovirt-vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("Import of VM ", func() {
	var (
		f           = framework.NewFrameworkOrDie("networking")
		secret      corev1.Secret
		namespace   string
		networkName string
	)

	BeforeSuite(func() {
		err := f.ConfigureNodeNetwork()
		if err != nil {
			Fail(err.Error())
		}
	})

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
		nad, err := f.CreateLinuxBridgeNetworkAttachmentDefinition()
		if err != nil {
			Fail(err.Error())
		}
		networkName = nad.Name
	})

	Context("with multus network", func() {
		It("should create running VM", func() {
			vmi := utils.VirtualMachineImportCr(vms.BasicNetworkVmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
				NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &multusType, Target: v2vv1alpha1.ObjectIdentifier{
						Name:      networkName,
						Namespace: &f.Namespace.Name,
					}},
				},
			}
			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("Having correct network configuration")
			vm, _ := f.KubeVirtClient.VirtualMachineInstance(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			spec := vm.Spec
			Expect(spec.Networks).To(HaveLen(1))
			Expect(spec.Networks[0].Multus).ToNot(BeNil())
			Expect(spec.Networks[0].Multus.NetworkName).To(BeEquivalentTo(networkName))

			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(1))
			nic := spec.Domain.Devices.Interfaces[0]
			Expect(nic.Name).To(BeEquivalentTo(spec.Networks[0].Name))
			Expect(nic.MacAddress).To(BeEquivalentTo(vms.BasicNetworkVmNicMAC))
			Expect(nic.Bridge).ToNot(BeNil())
			Expect(nic.Model).To(BeEquivalentTo("virtio"))

			Expect(vm.Status.Interfaces).To(HaveLen(1))
			Expect(vm.Status.Interfaces[0].Name).To(BeEquivalentTo(spec.Networks[0].Name))
			Expect(vm.Status.Interfaces[0].MAC).To(BeEquivalentTo(vms.BasicNetworkVmNicMAC))
		})
	})
})
