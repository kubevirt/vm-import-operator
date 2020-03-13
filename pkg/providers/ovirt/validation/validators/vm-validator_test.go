package validators

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

var _ = Describe("Validating VM", func() {
	It("should accept vm ", func() {
		var vm = newVM()

		failures := ValidateVM(vm)

		Expect(failures).To(BeEmpty())
	})
	It("should flag vm with boot menu enabled ", func() {
		var vm = newVM()
		bios := vm.MustBios()
		bootMenu := ovirtsdk.BootMenu{}
		bootMenu.SetEnabled(true)
		bios.SetBootMenu(&bootMenu)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMBiosBootMenuID))
	})
	It("should flag vm with no bios type ", func() {
		var vm = newVM()
		bios := ovirtsdk.Bios{}
		vm.SetBios(&bios)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMBiosTypeID))
	})
	It("should flag vm with q35_secure_boot bios ", func() {
		var vm = newVM()
		bios := vm.MustBios()
		bios.SetType("q35_secure_boot")

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMBiosTypeQ35SecureBootID))
	})
	It("should flag vm with s390x CPU ", func() {
		var vm = newVM()
		cpu := ovirtsdk.Cpu{}
		cpu.SetArchitecture("s390x")
		vm.SetCpu(&cpu)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMCpuArchitectureID))
	})
	table.DescribeTable("should flag CPU with illegal pinning for", func(pins []*ovirtsdk.VcpuPin) {
		vm := newVM()
		vm.MustCpu().MustCpuTune().MustVcpuPins().SetSlice(pins)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMCpuTuneID))
	},
		table.Entry("duplicate pins", []*ovirtsdk.VcpuPin{newCPUPin(0, "0"), newCPUPin(1, "0")}),
		table.Entry("cpu range", []*ovirtsdk.VcpuPin{newCPUPin(0, "0-1"), newCPUPin(1, "0-1")}),
		table.Entry("cpu set", []*ovirtsdk.VcpuPin{newCPUPin(0, "0,1"), newCPUPin(1, "0,1")}),
		table.Entry("cpu exclusion", []*ovirtsdk.VcpuPin{newCPUPin(0, "^1")}),
	)
	It("should flag vm with CPU shares ", func() {
		var vm = newVM()
		vm.SetCpuShares(1024)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMCpuSharesID))
	})
	It("should flag vm with custom properties ", func() {
		var vm = newVM()
		cps := ovirtsdk.CustomPropertySlice{}
		p1 := ovirtsdk.CustomProperty{}
		properties := []*ovirtsdk.CustomProperty{&p1}
		cps.SetSlice(properties)
		vm.SetCustomProperties(&cps)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMCustomPropertiesID))
	})
	It("should flag vm with spice display ", func() {
		var vm = newVM()
		display := ovirtsdk.Display{}
		display.SetType("spice")
		vm.SetDisplay(&display)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMDisplayTypeID))
	})
	It("should flag vm with illegal images ", func() {
		var vm = newVM()
		vm.SetHasIllegalImages(true)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMHasIllegalImagesID))
	})
	It("should flag vm with high availability priority ", func() {
		var vm = newVM()
		vm.MustHighAvailability().SetPriority(1)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMHighAvailabilityPriorityID))
	})
	It("should flag vm with IO Threads configured ", func() {
		var vm = newVM()
		io := ovirtsdk.Io{}
		io.SetThreads(4)
		vm.SetIo(&io)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMIoThreadsID))
	})
	It("should flag vm with memory balooning ", func() {
		var vm = newVM()
		memPolicy := ovirtsdk.MemoryPolicy{}
		memPolicy.SetBallooning(true)
		vm.SetMemoryPolicy(&memPolicy)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMMemoryPolicyBallooningID))
	})
	It("should flag vm with guaranteed memory ", func() {
		var vm = newVM()
		memPolicy := ovirtsdk.MemoryPolicy{}
		memPolicy.SetGuaranteed(1024)
		vm.SetMemoryPolicy(&memPolicy)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMMemoryPolicyGuaranteedID))
	})
	It("should flag vm with overcommit percent ", func() {
		var vm = newVM()
		memPolicy := ovirtsdk.MemoryPolicy{}
		memOverCommit := ovirtsdk.MemoryOverCommit{}
		memOverCommit.SetPercent(10)
		memPolicy.SetOverCommit(&memOverCommit)
		vm.SetMemoryPolicy(&memPolicy)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMMemoryPolicyOvercommitPercentID))
	})
	It("should flag vm with migration options ", func() {
		var vm = newVM()
		migration := ovirtsdk.MigrationOptions{}
		vm.SetMigration(&migration)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMMigrationID))
	})
	It("should flag vm with migration downtime ", func() {
		var vm = newVM()
		vm.SetMigrationDowntime(5)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMMigrationDowntimeID))
	})
	It("should flag vm with NUMA tune mode ", func() {
		var vm = newVM()
		vm.SetNumaTuneMode("strict")

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMNumaTuneModeID))
	})
	It("should flag vm with origin == kubevirt ", func() {
		var vm = newVM()
		vm.SetOrigin("kubevirt")

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMOriginID))
	})
	table.DescribeTable("should flag VM with illegal random number generator source", func(source string) {
		vm := newVM()
		vm.MustRngDevice().SetSource(ovirtsdk.RngSource(source))

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMRngDeviceSourceID))
	},
		table.Entry("hwrng", "hwrng"),
		table.Entry("random", "random"),

		table.Entry("garbage", "safdwlfkq332"),
		table.Entry("empty", ""),
	)
	It("should flag vm with sound card enabled", func() {
		var vm = newVM()
		vm.SetSoundcardEnabled(true)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMSoundcardEnabledID))
	})
	It("should flag vm with start paused enabled", func() {
		var vm = newVM()
		vm.SetStartPaused(true)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMStartPausedID))
	})
	It("should flag vm with storage error resume behaviour specified", func() {
		var vm = newVM()
		vm.SetStorageErrorResumeBehaviour("auto_resume")

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMStorageErrorResumeBehaviourID))
	})
	It("should flag vm with tunnel migration enabled", func() {
		var vm = newVM()
		vm.SetTunnelMigration(true)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMTunnelMigrationID))
	})
	It("should flag vm with USB configured", func() {
		var vm = newVM()
		usb := ovirtsdk.Usb{}
		vm.SetUsb(&usb)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMUsbID))
	})
	It("should flag vm with spice console configured", func() {
		var vm = newVM()
		consoles := []*ovirtsdk.GraphicsConsole{newGraphicsConsole("spice"), newGraphicsConsole("vnc")}
		vm.MustGraphicsConsoles().SetSlice(consoles)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMGraphicConsolesID))
	})
	It("should flag vm's host devices", func() {
		var vm = newVM()
		devices := []*ovirtsdk.HostDevice{&ovirtsdk.HostDevice{}}
		hostDevices := ovirtsdk.HostDeviceSlice{}
		hostDevices.SetSlice(devices)
		vm.SetHostDevices(&hostDevices)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMHostDevicesID))
	})
	It("should flag vm's reported devices", func() {
		var vm = newVM()
		devices := []*ovirtsdk.ReportedDevice{&ovirtsdk.ReportedDevice{}}
		reportedDevices := ovirtsdk.ReportedDeviceSlice{}
		reportedDevices.SetSlice(devices)
		vm.SetReportedDevices(&reportedDevices)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMReportedDevicesID))
	})
	It("should flag vm with quota", func() {
		var vm = newVM()
		quota := ovirtsdk.Quota{}
		quota.SetId("quota_id")
		vm.SetQuota(&quota)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMQuotaID))
	})
	It("should flag illegal watchdog - diag288", func() {
		var vm = newVM()
		watchdog := ovirtsdk.Watchdog{}
		watchdog.SetModel("diag288")
		vm.MustWatchdogs().SetSlice([]*ovirtsdk.Watchdog{&watchdog})

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMWatchdogsID))
	})
	It("should flag CD ROM with image stored in non-data domain", func() {
		var vm = newVM()
		storageDomain := ovirtsdk.StorageDomain{}
		storageDomain.SetType("iso")
		file := ovirtsdk.File{}
		file.SetStorageDomain(&storageDomain)
		cdrom := ovirtsdk.Cdrom{}
		cdrom.SetId("cd_id")
		cdrom.SetFile(&file)
		cdroms := []*ovirtsdk.Cdrom{&cdrom}
		vm.MustCdroms().SetSlice(cdroms)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMCdromsID))
	})
	It("should flag floppies", func() {
		var vm = newVM()

		floppies := []*ovirtsdk.Floppy{&ovirtsdk.Floppy{}}
		floppySlice := ovirtsdk.FloppySlice{}
		floppySlice.SetSlice(floppies)
		vm.SetFloppies(&floppySlice)

		failures := ValidateVM(vm)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(VMFloppiesID))
	})
})

func newGraphicsConsole(protocol string) *ovirtsdk.GraphicsConsole {
	console := ovirtsdk.GraphicsConsole{}
	console.SetProtocol(ovirtsdk.GraphicsType(protocol))
	return &console
}

func newVM() *ovirtsdk.Vm {
	vm := ovirtsdk.Vm{}
	bios := ovirtsdk.Bios{}
	bios.SetType("q35_sea_bios")
	vm.SetBios(&bios)

	cpu := ovirtsdk.Cpu{}
	cpu.SetArchitecture("x86_64")
	cpuTune := ovirtsdk.CpuTune{}
	pinSlice := ovirtsdk.VcpuPinSlice{}
	pins := []*ovirtsdk.VcpuPin{newCPUPin(0, "0"), newCPUPin(1, "1")}
	pinSlice.SetSlice(pins)
	cpuTune.SetVcpuPins(&pinSlice)
	cpu.SetCpuTune(&cpuTune)
	vm.SetCpu(&cpu)

	ha := ovirtsdk.HighAvailability{}
	ha.SetEnabled(true)
	vm.SetHighAvailability(&ha)

	vm.SetOrigin("ovirt")

	rng := ovirtsdk.RngDevice{}
	rng.SetSource("urandom")

	gfxConsoles := ovirtsdk.GraphicsConsoleSlice{}
	consoles := []*ovirtsdk.GraphicsConsole{newGraphicsConsole("vnc")}
	gfxConsoles.SetSlice(consoles)
	vm.SetGraphicsConsoles(&gfxConsoles)
	vm.SetRngDevice(&rng)

	watchdog := ovirtsdk.Watchdog{}
	watchdog.SetModel("i6300esb")
	wdSlice := ovirtsdk.WatchdogSlice{}
	wdSlice.SetSlice([]*ovirtsdk.Watchdog{&watchdog})
	vm.SetWatchdogs(&wdSlice)

	storageDomain := ovirtsdk.StorageDomain{}
	storageDomain.SetType("data")
	file := ovirtsdk.File{}
	file.SetStorageDomain(&storageDomain)
	cdrom := ovirtsdk.Cdrom{}
	cdrom.SetFile(&file)
	cdroms := []*ovirtsdk.Cdrom{&cdrom}
	cdromSlice := ovirtsdk.CdromSlice{}
	cdromSlice.SetSlice(cdroms)
	vm.SetCdroms(&cdromSlice)

	return &vm
}
func newCPUPin(cpu int64, cpuSet string) *ovirtsdk.VcpuPin {
	pin := ovirtsdk.VcpuPin{}
	pin.SetVcpu(cpu)
	pin.SetCpuSet(cpuSet)
	return &pin
}
