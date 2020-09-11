package os_test

import (
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/os"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	ovirtsdk "github.com/ovirt/go-ovirt"

	. "github.com/onsi/gomega"
)

var (
	getOSMaps func() (map[string]string, map[string]string, error)
	finder    = os.OVirtOSFinder{OsMapProvider: &mockOsMapProvider{}}
)

var _ = Describe("OS finder ", func() {
	BeforeEach(func() {
		getOSMaps = func() (map[string]string, map[string]string, error) {
			guest2common := map[string]string{"Red Hat Enterprise Linux Server": "rhel"}
			os2common := map[string]string{"rhel_6": "rhel6.9"}
			return guest2common, os2common, nil
		}
	})

	It("should find OS from Guest OS information", func() {
		vm := ovirtsdk.NewVmBuilder().
			GuestOperatingSystemBuilder(
				ovirtsdk.NewGuestOperatingSystemBuilder().
					Distribution("Red Hat Enterprise Linux Server").
					VersionBuilder(ovirtsdk.NewVersionBuilder().FullVersion("7.7"))).
			MustBuild()

		os, err := finder.FindOperatingSystem(vm)

		Expect(err).ToNot(HaveOccurred())
		Expect(os).To(BeEquivalentTo("rhel7.7"))
	})

	It("should find OS from OS type and mapping present", func() {
		vm := ovirtsdk.NewVmBuilder().
			OsBuilder(
				ovirtsdk.NewOperatingSystemBuilder().
					Type("rhel_6")).
			MustBuild()

		os, err := finder.FindOperatingSystem(vm)

		Expect(err).ToNot(HaveOccurred())
		Expect(os).To(BeEquivalentTo("rhel6.9"))
	})

	table.DescribeTable("should find default OS from OS type and no mapping for the type", func(osType string, expectedOs string) {
		vm := ovirtsdk.NewVmBuilder().
			OsBuilder(
				ovirtsdk.NewOperatingSystemBuilder().
					Type(osType)).
			MustBuild()

		os, err := finder.FindOperatingSystem(vm)

		Expect(err).ToNot(HaveOccurred())
		Expect(os).To(BeEquivalentTo(expectedOs))
	},
		table.Entry("for generic Linux", "linux X", "rhel8.2"),
		table.Entry("for RHEL", "rhel X", "rhel8.2"),

		table.Entry("for generic Windows", "windows", "win10"),
		table.Entry("for windows_7", "windows_7", "win10"),
	)

	It("should return error for os map provider error", func() {
		getOSMaps = func() (map[string]string, map[string]string, error) {
			zero := map[string]string{}
			return zero, zero, fmt.Errorf("Boom!")
		}

		vm := ovirtsdk.NewVmBuilder().
			OsBuilder(
				ovirtsdk.NewOperatingSystemBuilder().
					Type("linux")).
			MustBuild()

		_, err := finder.FindOperatingSystem(vm)

		Expect(err).To(HaveOccurred())
	})

	It("should return error for no OS found", func() {
		vm := ovirtsdk.NewVmBuilder().MustBuild()

		_, err := finder.FindOperatingSystem(vm)

		Expect(err).To(HaveOccurred())
	})
})

type mockOsMapProvider struct{}

func (m *mockOsMapProvider) GetOSMaps() (map[string]string, map[string]string, error) {
	return getOSMaps()
}
