package mapper_test

import (
	"fmt"

	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	kubevirtv1 "kubevirt.io/client-go/api/v1"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
)

const memoryGI = 1024 * 1024 * 1024

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

	It("should map name", func() {
		Expect(vmSpec.Name).To(Equal(vm.MustName()))
	})

	It("should map CPU topology", func() {
		vmTopology := vm.MustCpu().MustTopology()
		vmSpecCPU := vmSpec.Spec.Template.Spec.Domain.CPU
		Expect(vmSpecCPU.Cores).To(Equal(uint32(vmTopology.MustCores())))
		Expect(vmSpecCPU.Sockets).To(Equal(uint32(vmTopology.MustSockets())))
		Expect(vmSpecCPU.Threads).To(Equal(uint32(vmTopology.MustThreads())))
	})

	It("should map BIOS", func() {
		bootloader := vmSpec.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS
		Expect(bootloader).ToNot(BeNil())
	})

	It("should map HA", func() {
		Expect(*vmSpec.Spec.Running).To(Equal(vm.MustHighAvailability().MustEnabled()))
	})

	It("should map Memory", func() {
		// guest memory
		guestMemory, _ := vmSpec.Spec.Template.Spec.Domain.Memory.Guest.AsInt64()
		Expect(guestMemory).To(Equal(vm.MustMemory()))

		// huge page
		hugePageSize := vmSpec.Spec.Template.Spec.Domain.Memory.Hugepages.PageSize
		mappedHugePageSize := vm.MustCustomProperties().Slice()[0].MustValue()
		Expect(hugePageSize).To(Equal(fmt.Sprintf("%sMi", mappedHugePageSize)))

		// max memory
		memLimit, _ := vmSpec.Spec.Template.Spec.Domain.Resources.Limits.Memory().AsInt64()
		memMax := vm.MustMemoryPolicy().MustMax()
		Expect(memLimit).To(Equal(memMax))
	})

	It("should map graphics consoles", func() {
		graphicsEnabled := vmSpec.Spec.Template.Spec.Domain.Devices.AutoattachGraphicsDevice
		Expect(*graphicsEnabled).To(Equal(len(vm.MustGraphicsConsoles().Slice()) > 0))
	})

	It("should map timezone", func() {
		timezone := vmSpec.Spec.Template.Spec.Domain.Clock.Timezone
		Expect(string(*timezone)).To(Equal(vm.MustTimeZone().MustName()))
	})

	It("should map placement policy", func() {
		evictionStrategy := vmSpec.Spec.Template.Spec.EvictionStrategy
		Expect(string(*evictionStrategy)).ToNot(BeNil())
	})

	It("should map annotations", func() {
		annotations := vmSpec.ObjectMeta.Annotations
		Expect(annotations[mapper.AnnotationComment]).To(Equal(vm.MustComment()))
		Expect(annotations[mapper.AnnotationSso]).To(Equal(string(vm.MustSso().MustMethods().Slice()[0].MustId())))
	})

	It("should map labels", func() {
		labels := vmSpec.ObjectMeta.Labels
		Expect(labels[mapper.LabelOrigin]).To(Equal(vm.MustOrigin()))
		Expect(labels[mapper.LabelInstanceType]).To(Equal(vm.MustInstanceType().MustName()))
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
		HighAvailability(
			ovirtsdk.NewHighAvailabilityBuilder().
				Enabled(true).
				MustBuild()).
		Memory(memoryGI).
		MemoryPolicy(
			ovirtsdk.NewMemoryPolicyBuilder().
				Max(memoryGI).MustBuild()).
		GraphicsConsolesOfAny(
			ovirtsdk.NewGraphicsConsoleBuilder().
				Name("testConsole").MustBuild()).
		TimeZone(
			ovirtsdk.NewTimeZoneBuilder().
				Name(">Etc/GMT").MustBuild()).
		PlacementPolicy(
			ovirtsdk.NewVmPlacementPolicyBuilder().
				Affinity(ovirtsdk.VMAFFINITY_MIGRATABLE).MustBuild()).
		Origin("ovirt").
		Comment("vmcomment").
		InstanceType(
			ovirtsdk.NewInstanceTypeBuilder().
				Name("server").MustBuild()).
		TagsOfAny(
			ovirtsdk.NewTagBuilder().
				Name("mytag").MustBuild()).
		CustomPropertiesOfAny(
			ovirtsdk.NewCustomPropertyBuilder().
				Name("hugepages").
				Value("2048").MustBuild()).
		Sso(
			ovirtsdk.NewSsoBuilder().
				MethodsOfAny(
					ovirtsdk.NewMethodBuilder().
						Id(ovirtsdk.SSOMETHOD_GUEST_AGENT).
						MustBuild()).
				MustBuild()).
		MustBuild()
}
