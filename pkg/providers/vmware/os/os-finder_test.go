package os_test

import (
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	"github.com/onsi/ginkgo/extensions/table"
	"github.com/vmware/govmomi/simulator"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	getOSMaps func() (map[string]string, map[string]string, error)
	finder    = os.VmwareOSFinder{OsMapProvider: &mockOsMapProvider{}}
)

var _ = Describe("OS finder ", func() {
	BeforeEach(func() {
		getOSMaps = func() (map[string]string, map[string]string, error) {
			guest2common := map[string]string{}
			os2common := map[string]string{"rhel6Guest": "rhel6.9"}
			return guest2common, os2common, nil
		}
	})

	It("should find OS from Guest.GuestId", func() {
		model := simulator.VPX()
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()

		vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		vm.VirtualMachine.Summary.Guest.GuestId = "rhel6Guest"

		os, err := finder.FindOperatingSystem(&vm.VirtualMachine)

		Expect(err).ToNot(HaveOccurred())
		Expect(os).To(BeEquivalentTo("rhel6.9"))
	})

	It("should find OS from Config.GuestId if Guest.GuestId isn't available", func() {
		model := simulator.VPX()
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()

		vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		vm.VirtualMachine.Summary.Config.GuestId = "rhel6Guest"

		os, err := finder.FindOperatingSystem(&vm.VirtualMachine)

		Expect(err).ToNot(HaveOccurred())
		Expect(os).To(BeEquivalentTo("rhel6.9"))
	})

	table.DescribeTable("should try to determine linux or windows from the GuestFullName if GuestId isn't in the map", func(fullName string, expectedOs string) {
		model := simulator.VPX()
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()

		vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		vm.VirtualMachine.Summary.Config.GuestFullName = fullName

		os, err := finder.FindOperatingSystem(&vm.VirtualMachine)

		Expect(err).ToNot(HaveOccurred())
		Expect(os).To(BeEquivalentTo(expectedOs))
	},
		table.Entry("for generic Linux", "linux X", "rhel8"),
		table.Entry("for RHEL", "rhel X", "rhel8"),

		table.Entry("for generic Windows", "windows", "windows"),
		table.Entry("for windows_7", "windows_7", "windows"),
	)

	It("should return error for os map provider error", func() {
		getOSMaps = func() (map[string]string, map[string]string, error) {
			zero := map[string]string{}
			return zero, zero, fmt.Errorf("Boom!")
		}

		model := simulator.VPX()
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()

		vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		vm.Summary.Config.GuestId = "rhel6Guest"

		_, err := finder.FindOperatingSystem(&vm.VirtualMachine)

		Expect(err).To(HaveOccurred())
	})

	It("should return error for no OS found", func() {
		model := simulator.VPX()
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()

		vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		vm.Summary.Guest.GuestId = "invalid"
		vm.Summary.Config.GuestId = "invalid"
		vm.Summary.Config.GuestFullName = "invalid"

		_, err := finder.FindOperatingSystem(&vm.VirtualMachine)

		Expect(err).To(HaveOccurred())
	})
})

type mockOsMapProvider struct{}

func (m *mockOsMapProvider) GetOSMaps() (map[string]string, map[string]string, error) {
	return getOSMaps()
}
