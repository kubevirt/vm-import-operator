package mapper_test

import (
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	kubevirtv1 "kubevirt.io/client-go/api/v1"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
)

var targetVMName = "myvm"

var _ = Describe("Test mapping virtual machine attributes", func() {
	var (
		vm     *ovirtsdk.Vm
		vmSpec *kubevirtv1.VirtualMachine
	)

	BeforeEach(func() {
		vm = createVM()
		mapper := mapper.NewOvirtMapper(vm, nil, provider.DataVolumeCredentials{}, "")
		vmSpec = mapper.MapVM(&targetVMName)
	})

	It("For name", func() {
		Expect(vmSpec.Name).To(Equal(vm.MustName()))
	})

	It("For CPU topology", func() {
		vmTopology := vm.MustCpu().MustTopology()
		vmSpecCPU := vmSpec.Spec.Template.Spec.Domain.CPU
		Expect(vmSpecCPU.Cores).To(Equal(uint32(vmTopology.MustCores())))
		Expect(vmSpecCPU.Sockets).To(Equal(uint32(vmTopology.MustSockets())))
		Expect(vmSpecCPU.Threads).To(Equal(uint32(vmTopology.MustThreads())))
	})

	It("For BIOS", func() {
		bootloader := vmSpec.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS
		Expect(bootloader).ToNot(BeNil())
	})
})

func createVM() *ovirtsdk.Vm {
	return ovirtsdk.NewVmBuilder().
		Name("myvm").
		Bios(
			ovirtsdk.NewBiosBuilder().
				Type(ovirtsdk.BIOSTYPE_Q35_SEA_BIOS).MustBuild()).
		Cpu(
			ovirtsdk.NewCpuBuilder().
				Topology(
					ovirtsdk.NewCpuTopologyBuilder().
						Cores(1).
						Sockets(2).
						Threads(4).
						MustBuild()).
				MustBuild()).
		MustBuild()
}
