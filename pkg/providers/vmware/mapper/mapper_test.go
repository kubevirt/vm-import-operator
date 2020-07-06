package mapper_test

import (
	"context"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/mapper"
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/apimachinery/pkg/api/resource"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

var (
	targetVMName = "import-test"
	memoryReservationStr = "2048Mi"
	cpuCores int32 = 3
	cpuSockets int32 = 3
	gmtOffsetSeconds int32 = 0
	vmMoRef = "vm-2782"
	vmPodNetworkName = "VM Network"
	vmPodNetworkMac = "00:50:56:a5:d4:be"
	vmPodNetworkNormalizedName, _ = utils.NormalizeName(vmPodNetworkName)
	vmMultusNetworkName = "VM_10G_Network"
	vmMultusNetworkNormalizedName, _ = utils.NormalizeName(vmMultusNetworkName)
	vmMultusNetworkMac = "00:50:56:a5:e2:e8"
	multusNetwork = "multus"
	podNetwork = "pod"
)

type mockOsFinder struct {}
func (r mockOsFinder) FindOperatingSystem(_ *mo.VirtualMachine) (string, error) {
	return findOs()
}

var (
	osFinder os.OSFinder = mockOsFinder{}
	findOs func() (string, error)
	)


var _ = Describe("Test mapping virtual machine attributes", func() {
	var (
		vm *object.VirtualMachine
		vmProperties *mo.VirtualMachine
		mappings *v2vv1alpha1.VmwareMappings
		vmSpec *kubevirtv1.VirtualMachine
		credentials *mapper.DataVolumeCredentials
	)

	BeforeEach(func() {
		model := simulator.VPX()
		err := model.Load("../../../../tests/vmware/sim")
		Expect(err).To(BeNil())

		server := model.Service.NewServer()
		client, _ := govmomi.NewClient(context.TODO(), server.URL, false)

		findOs = func() (string, error) {
			return "linux", nil
		}

		moRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmMoRef}
		vm = object.NewVirtualMachine(client.Client, moRef)
		vmProperties = &mo.VirtualMachine{}
		err = vm.Properties(context.TODO(), vm.Reference(), nil, vmProperties)
		Expect(err).To(BeNil())
		host := object.NewHostSystem(client.Client, *vmProperties.Runtime.Host)
		hostProperties := &mo.HostSystem{}
		err = host.Properties(context.TODO(), host.Reference(), nil, hostProperties)
		Expect(err).To(BeNil())

		username := server.URL.User.Username()
		password, _ := server.URL.User.Password()
		credentials = &mapper.DataVolumeCredentials{
			URL:        server.URL.String(),
			Username:   username,
			Password:   password,
			Thumbprint: "",
			SecretName: "",
		}
		mappings = createMappings()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err = vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())
	})

	It("should map name", func() {
		Expect(vmSpec.Name).To(Equal(vmProperties.Config.Name))
	})

	It("should map memory reservation", func() {
		quantity, _ := resource.ParseQuantity(memoryReservationStr)
		Expect(vmSpec.Spec.Template.Spec.Domain.Resources.Requests.Memory().Value()).To(Equal(quantity.Value()))
	})

	It("should map CPU topology", func() {
		Expect(int32(vmSpec.Spec.Template.Spec.Domain.CPU.Cores)).To(Equal(cpuCores))
		Expect(int32(vmSpec.Spec.Template.Spec.Domain.CPU.Sockets)).To(Equal(cpuSockets))

	})

	It("should map timezone", func() {
		Expect(int32(*vmSpec.Spec.Template.Spec.Domain.Clock.UTC.OffsetSeconds)).To(Equal(gmtOffsetSeconds))
	})

	It("should map networks", func() {
		interfaces := vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces
		networks := vmSpec.Spec.Template.Spec.Networks
		networkMapping := *mappings.NetworkMappings

		// interface to be connected to a pod network
		Expect(interfaces[0].Name).To(Equal(vmPodNetworkNormalizedName))
		Expect(interfaces[0].Bridge).To(BeNil())
		Expect(interfaces[0].Masquerade).To(Not(BeNil()))
		Expect(interfaces[0].MacAddress).To(Equal(vmPodNetworkMac))
		Expect(networks[0].Name).To(Equal(vmPodNetworkNormalizedName))
		Expect(networks[0].Pod).To(Not(BeNil()))

		// interface to be connected to a multus network
		Expect(interfaces[1].Name).To(Equal(vmMultusNetworkNormalizedName))
		Expect(interfaces[1].Bridge).To(Not(BeNil()))
		Expect(interfaces[1].MacAddress).To(Equal(vmMultusNetworkMac))
		Expect(networks[1].Name).To(Equal(vmMultusNetworkNormalizedName))
		Expect(networks[1].Multus.NetworkName).To(Equal(networkMapping[0].Target.Name))
	})
})

var _ = Describe("Test mapping disks", func() {
	var (
		vm *object.VirtualMachine
		vmProperties *mo.VirtualMachine
		hostProperties *mo.HostSystem
		credentials *mapper.DataVolumeCredentials
	)

	BeforeEach(func() {
		model := simulator.VPX()
		err := model.Load("../../../../tests/vmware/sim")
		Expect(err).To(BeNil())
		server := model.Service.NewServer()
		client, _ := govmomi.NewClient(context.TODO(), server.URL, false)
		moRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmMoRef}
		vm = object.NewVirtualMachine(client.Client, moRef)
		vmProperties = &mo.VirtualMachine{}
		err = vm.Properties(context.TODO(), vm.Reference(), nil, vmProperties)
		Expect(err).To(BeNil())
		host := object.NewHostSystem(client.Client, *vmProperties.Runtime.Host)
		hostProperties := &mo.HostSystem{}
		err = host.Properties(context.TODO(), host.Reference(), nil, hostProperties)
		Expect(err).To(BeNil())

		username := server.URL.User.Username()
		password, _ := server.URL.User.Password()
		credentials = &mapper.DataVolumeCredentials{
			URL:        server.URL.String(),
			Username:   username,
			Password:   password,
			Thumbprint: "",
			SecretName: "",
		}
	})

	It("should map disks", func() {
		mappings := createMappings()
		namespace := "my-namespace"
		mapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, namespace, osFinder)
		dvs, _ := mapper.MapDataVolumes(&targetVMName)
		Expect(dvs).To(HaveLen(2))
		Expect(dvs).To(HaveKey("import-test-421-2000"))
		Expect(dvs).To(HaveKey("import-test-421-2001"))
	})
})

func createMappings() *v2vv1alpha1.VmwareMappings {
	var networks []v2vv1alpha1.ResourceMappingItem
	networks = append(networks,
		v2vv1alpha1.ResourceMappingItem{
			Source: v2vv1alpha1.Source{
				Name: &vmMultusNetworkName,
			},
			Target: v2vv1alpha1.ObjectIdentifier{
				Name: "net-attach-def",
			},
			Type: &multusNetwork,
		},
		v2vv1alpha1.ResourceMappingItem{
			Source: v2vv1alpha1.Source{
				Name: &vmPodNetworkName,
			},
			Type: &podNetwork,
		})

	return &v2vv1alpha1.VmwareMappings{
		NetworkMappings: &networks,
		DiskMappings: &[]v2vv1alpha1.ResourceMappingItem{},
	}
}