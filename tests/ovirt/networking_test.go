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

type networkingTest struct {
	framework *fwk.Framework
}

var _ = Describe("Import of VM ", func() {
	var (
		f           = fwk.NewFrameworkOrDie("networking")
		test        = networkingTest{f}
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
			vmID := vms.BasicNetworkVmID
			vmXml := f.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
			nicsXml := f.LoadFile("nics/one.xml")
			stubbing := test.prepareCommonSubResources(vmID).
				StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
				StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
				Build()
			err := f.OvirtStubbingClient.Stub(stubbing)
			Expect(err).To(BeNil())

			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
				NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.MultusType, Target: v2vv1alpha1.ObjectIdentifier{
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

			By("having correct network configuration")
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

	Context("with two networks: Multus and Pod", func() {
		It("should create running VM", func() {
			vmID := vms.TwoNetworksVmID
			vmXml := f.LoadFile("vms/two-networks-vm.xml")
			nicsXml := f.LoadFile("nics/two.xml")
			network2Xml := f.LoadFile("networks/net-2.xml")
			vnicProfile2Xml := f.LoadFile("vnic-profiles/vnic-profile-2.xml")
			stubbing := test.prepareCommonSubResources(vmID).
				StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
				StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
				StubGet("/ovirt-engine/api/networks/net-2", &network2Xml).
				StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-2", &vnicProfile2Xml).
				Build()
			err := f.OvirtStubbingClient.Stub(stubbing)
			Expect(err).To(BeNil())

			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
				NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &vms.VNicProfile1ID}, Type: &tests.MultusType, Target: v2vv1alpha1.ObjectIdentifier{
						Name:      networkName,
						Namespace: &f.Namespace.Name,
					}},
					{Source: v2vv1alpha1.Source{ID: &vms.VNicProfile2ID}, Type: &tests.PodType},
				},
			}
			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("having correct network configurations")
			vm, _ := f.KubeVirtClient.VirtualMachineInstance(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			spec := vm.Spec
			Expect(spec.Networks).To(HaveLen(2))
			net1 := spec.Networks[0]
			net2 := spec.Networks[1]
			if net1.Pod != nil {
				net1, net2 = net2, net1
			}
			By("having one Multus network")
			Expect(net1.Multus).ToNot(BeNil())
			Expect(net1.Multus.NetworkName).To(BeEquivalentTo(networkName))
			Expect(net1.Pod).To(BeNil())

			By("having one Pod network")
			Expect(net2.Pod).ToNot(BeNil())
			Expect(net2.Multus).To(BeNil())

			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(2))
			nic1 := spec.Domain.Devices.Interfaces[0]
			nic2 := spec.Domain.Devices.Interfaces[1]
			if nic1.Masquerade != nil {
				nic1, nic2 = nic2, nic1
			}
			By("having one bridge interface")
			Expect(nic1.Name).To(BeEquivalentTo(net1.Name))
			Expect(nic1.MacAddress).To(BeEquivalentTo(vms.BasicNetworkVmNicMAC))
			Expect(nic1.Bridge).ToNot(BeNil())
			Expect(nic1.Masquerade).To(BeNil())
			Expect(nic1.Model).To(BeEquivalentTo("virtio"))

			By("having one masquarade interface")
			Expect(nic2.Name).To(BeEquivalentTo(net2.Name))
			Expect(nic2.MacAddress).To(BeEquivalentTo(vms.Nic2MAC))
			Expect(nic2.Masquerade).ToNot(BeNil())
			Expect(nic2.Bridge).To(BeNil())
			Expect(nic2.Model).To(BeEquivalentTo("virtio"))

			By("having two interfaces reported in the status")
			Expect(vm.Status.Interfaces).To(HaveLen(2))
		})
	})

	Context("with two Multus networks", func() {
		It("should create running VM", func() {
			vmID := vms.TwoNetworksVmID
			vmXml := f.LoadFile("vms/two-networks-vm.xml")
			nicsXml := f.LoadFile("nics/two.xml")
			network2Xml := f.LoadFile("networks/net-2.xml")
			vnicProfile2Xml := f.LoadFile("vnic-profiles/vnic-profile-2.xml")
			stubbing := test.prepareCommonSubResources(vmID).
				StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
				StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml).
				StubGet("/ovirt-engine/api/networks/net-2", &network2Xml).
				StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-2", &vnicProfile2Xml).
				Build()
			err := f.OvirtStubbingClient.Stub(stubbing)
			Expect(err).To(BeNil())

			By("having additional linux bridge network")
			nad2, err := f.CreateLinuxBridgeNetworkAttachmentDefinition()
			if err != nil {
				Fail(err.Error())
			}
			network2Name := nad2.Name

			vmi := utils.VirtualMachineImportCr(vmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.Source.Ovirt.Mappings = &v2vv1alpha1.OvirtMappings{
				NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{
						Source: v2vv1alpha1.Source{ID: &vms.VNicProfile1ID},
						Type:   &tests.MultusType,
						Target: v2vv1alpha1.ObjectIdentifier{
							Name:      networkName,
							Namespace: &f.Namespace.Name,
						}},
					{
						Source: v2vv1alpha1.Source{ID: &vms.VNicProfile2ID},
						Type:   &tests.MultusType,
						Target: v2vv1alpha1.ObjectIdentifier{
							Name:      network2Name,
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

			By("having correct network configurations")
			vm, _ := f.KubeVirtClient.VirtualMachineInstance(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			spec := vm.Spec
			Expect(spec.Networks).To(HaveLen(2))
			net1 := spec.Networks[0]
			net2 := spec.Networks[1]

			By("having two Multus network")
			Expect(net1.Multus).ToNot(BeNil())
			Expect(net2.Multus).ToNot(BeNil())
			if net1.Multus.NetworkName != networkName {
				net1, net2 = net2, net1
			}
			Expect(net1.Pod).To(BeNil())
			Expect(net1.Multus.NetworkName).To(BeEquivalentTo(networkName))
			Expect(net2.Pod).To(BeNil())
			Expect(net2.Multus.NetworkName).To(BeEquivalentTo(network2Name))

			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(2))
			nic1 := spec.Domain.Devices.Interfaces[0]
			nic2 := spec.Domain.Devices.Interfaces[1]
			if nic1.Name != net1.Name {
				nic1, nic2 = nic2, nic1
			}
			By("having interface #1 of type bridge")
			Expect(nic1.Name).To(BeEquivalentTo(net1.Name))
			Expect(nic1.MacAddress).To(BeEquivalentTo(vms.BasicNetworkVmNicMAC))
			Expect(nic1.Bridge).ToNot(BeNil())
			Expect(nic1.Masquerade).To(BeNil())
			Expect(nic1.Model).To(BeEquivalentTo("virtio"))

			By("having interface #2 of type bridge")
			Expect(nic2.Name).To(BeEquivalentTo(net2.Name))
			Expect(nic2.MacAddress).To(BeEquivalentTo(vms.Nic2MAC))
			Expect(nic2.Bridge).ToNot(BeNil())
			Expect(nic2.Masquerade).To(BeNil())
			Expect(nic2.Model).To(BeEquivalentTo("virtio"))

			By("having two interfaces reported in the status")
			Expect(vm.Status.Interfaces).To(HaveLen(2))
		})
	})
})

func (t *networkingTest) prepareCommonSubResources(vmID string) *sapi.StubbingBuilder {
	diskAttachmentsXml := t.framework.LoadFile("disk-attachments/one.xml")
	diskXml := t.framework.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "46137344"})
	domainXml := t.framework.LoadFile("storage-domains/domain-1.xml")
	consolesXml := t.framework.LoadFile("graphic-consoles/empty.xml")
	networkXml := t.framework.LoadFile("networks/net-1.xml")
	vnicProfileXml := t.framework.LoadFile("vnic-profiles/vnic-profile-1.xml")
	return sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml).
		StubGet("/ovirt-engine/api/networks/net-1", &networkXml).
		StubGet("/ovirt-engine/api/vnicprofiles/vnic-profile-1", &vnicProfileXml)
}
