package mapper_test

import (
	"context"
	"github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
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
	vmMoRef = "vm-70"
	targetVMName = "basic-vm"

	// vm attributes
	memoryReservationStr = "2048Mi"
	cpuCores int32 = 1
	cpuSockets int32 = 4
	gmtOffsetSeconds int32 = 3600

	// networks
	networkName = "VM Network"
	networkNormalizedName, _ = utils.NormalizeName(networkName)
	multusNetwork = "multus"
	podNetwork = "pod"

	// disks
	expectedNumDisks = 2
	expectedDiskName1 = "basic-vm-disk-202-0"
	expectedDiskName2 = "basic-vm-disk-202-1"
)

type mockOsFinder struct {}
func (r mockOsFinder) FindOperatingSystem(_ *mo.VirtualMachine) (string, error) {
	return findOs()
}

var (
	osFinder os.OSFinder = mockOsFinder{}
	findOs func() (string, error)
	)

func prepareVsphereObjects(client *govmomi.Client) (*object.VirtualMachine, *mo.VirtualMachine, *mo.HostSystem) {
	moRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmMoRef}
	vm := object.NewVirtualMachine(client.Client, moRef)
	vmProperties := &mo.VirtualMachine{}
	err := vm.Properties(context.TODO(), vm.Reference(), nil, vmProperties)
	Expect(err).To(BeNil())
	host := object.NewHostSystem(client.Client, *vmProperties.Runtime.Host)
	hostProperties := &mo.HostSystem{}
	err = host.Properties(context.TODO(), host.Reference(), nil, hostProperties)
	Expect(err).To(BeNil())
	// simulator hosts don't have a DateTimeInfo so set one
	hostProperties.Config.DateTimeInfo = &types.HostDateTimeInfo{
		DynamicData:         types.DynamicData{},
		TimeZone:            types.HostDateTimeSystemTimeZone{
			GmtOffset:   gmtOffsetSeconds,
		},
	}

	return vm, vmProperties, hostProperties
}

func prepareCredentials(server *simulator.Server) *mapper.DataVolumeCredentials {
	username := server.URL.User.Username()
	password, _ := server.URL.User.Password()
	return &mapper.DataVolumeCredentials{
		URL:        server.URL.String(),
		Username:   username,
		Password:   password,
		Thumbprint: "",
		SecretName: "",
	}
}

var _ = Describe("Test mapping virtual machine attributes", func() {
	var (
		vm *object.VirtualMachine
		vmProperties *mo.VirtualMachine
		hostProperties *mo.HostSystem
		credentials *mapper.DataVolumeCredentials
	)

	BeforeEach(func() {
		model := simulator.VPX()
		err := model.Load("../../../../tests/vmware/vcsim")
		Expect(err).To(BeNil())

		server := model.Service.NewServer()
		client, _ := govmomi.NewClient(context.TODO(), server.URL, false)

		findOs = func() (string, error) {
			return "linux", nil
		}

		vm, vmProperties, hostProperties = prepareVsphereObjects(client)
		credentials = prepareCredentials(server)
	})

	It("should map name", func() {
		mappings := createMinimalMapping()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err := vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())

		Expect(vmSpec.Name).To(Equal(vmProperties.Config.Name))
	})

	It("should map memory reservation", func() {
		mappings := createMinimalMapping()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err := vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())

		quantity, _ := resource.ParseQuantity(memoryReservationStr)
		Expect(vmSpec.Spec.Template.Spec.Domain.Resources.Requests.Memory().Value()).To(Equal(quantity.Value()))
	})

	It("should map CPU topology", func() {
		mappings := createMinimalMapping()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err := vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())

		Expect(int32(vmSpec.Spec.Template.Spec.Domain.CPU.Cores)).To(Equal(cpuCores))
		Expect(int32(vmSpec.Spec.Template.Spec.Domain.CPU.Sockets)).To(Equal(cpuSockets))

	})

	It("should map timezone", func() {
		mappings := createMinimalMapping()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err := vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())

		Expect(int32(*vmSpec.Spec.Template.Spec.Domain.Clock.UTC.OffsetSeconds)).To(Equal(gmtOffsetSeconds))
	})

	It("should map pod network", func() {
		mappings := createPodNetworkMapping()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err := vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())

		interfaces := vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces
		networks := vmSpec.Spec.Template.Spec.Networks

		// interface to be connected to a pod network
		Expect(interfaces[0].Name).To(Equal(networkNormalizedName))
		Expect(interfaces[0].Bridge).To(BeNil())
		Expect(interfaces[0].Masquerade).To(Not(BeNil()))
		Expect(networks[0].Name).To(Equal(networkNormalizedName))
		Expect(networks[0].Pod).To(Not(BeNil()))
	})

	It("should map multus network", func() {
		mappings := createMultusNetworkMapping()
		vmMapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		vmSpec, err := vmMapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
		Expect(err).To(BeNil())

		interfaces := vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces
		networks := vmSpec.Spec.Template.Spec.Networks
		networkMapping := *mappings.NetworkMappings


		// interface to be connected to a multus network
		Expect(interfaces[0].Name).To(Equal(networkNormalizedName))
		Expect(interfaces[0].Bridge).To(Not(BeNil()))
		Expect(networks[0].Name).To(Equal(networkNormalizedName))
		Expect(networks[0].Multus.NetworkName).To(Equal(networkMapping[0].Target.Name))
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
		err := model.Load("../../../../tests/vmware/vcsim")
		Expect(err).To(BeNil())
		server := model.Service.NewServer()
		client, _ := govmomi.NewClient(context.TODO(), server.URL, false)
		vm, vmProperties, hostProperties = prepareVsphereObjects(client)
		credentials = prepareCredentials(server)
	})

	It("should map disks", func() {
		mappings := createMinimalMapping()
		mapper := mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, mappings, "", osFinder)
		dvs, _ := mapper.MapDataVolumes(&targetVMName)
		Expect(dvs).To(HaveLen(expectedNumDisks))
		Expect(dvs).To(HaveKey(expectedDiskName1))
		Expect(dvs).To(HaveKey(expectedDiskName2))
	})
})

func createMinimalMapping() *v1beta1.VmwareMappings {
	return &v1beta1.VmwareMappings{
		NetworkMappings: &[]v1beta1.NetworkResourceMappingItem{},
		DiskMappings: &[]v1beta1.StorageResourceMappingItem{},
	}
}

func createMultusNetworkMapping() *v1beta1.VmwareMappings {
	var networks []v1beta1.NetworkResourceMappingItem
	networks = append(networks,
		v1beta1.NetworkResourceMappingItem{
			Source: v1beta1.Source{
				Name: &networkName,
			},
			Target: v1beta1.ObjectIdentifier{
				Name: "net-attach-def",
			},
			Type: &multusNetwork,
		})

	return &v1beta1.VmwareMappings{
		NetworkMappings: &networks,
		DiskMappings: &[]v1beta1.StorageResourceMappingItem{},
	}

}

func createPodNetworkMapping() *v1beta1.VmwareMappings {
	var networks []v1beta1.NetworkResourceMappingItem
	networks = append(networks,
		v1beta1.NetworkResourceMappingItem{
			Source: v1beta1.Source{
				Name: &networkName,
			},
			Type: &podNetwork,
		})

	return &v1beta1.VmwareMappings{
		NetworkMappings: &networks,
		DiskMappings: &[]v1beta1.StorageResourceMappingItem{},
	}
}