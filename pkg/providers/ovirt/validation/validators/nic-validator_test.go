package validators

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

var _ = Describe("Validating NIC", func() {
	table.DescribeTable("should flag nic with illegal interface model: ", func(iface string) {
		var nic = newNic()
		nic.SetInterface(ovirtsdk.NicInterface(iface))

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicInterfaceCheckID))
	},
		table.Entry("pci_passthrough", "pci_passthrough"),
		table.Entry("spapr_vlan", "spapr_vlan"),
		table.Entry("rtl8139_virtio", "rtl8139_virtio"),
		table.Entry("garbage", "lkfsldfksld3432432#$#@"),
		table.Entry("empty string", ""),
	)
	table.DescribeTable("should accept nic with legal interface model: ", func(iface string) {
		var nic = newNic()
		nic.SetInterface(ovirtsdk.NicInterface(iface))

		failures := validateNic(nic)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("virtio", "virtio"),
		table.Entry("e1000", "e1000"),
		table.Entry("rtl8139", "rtl8139"),
	)
	It("should flag nic without interface: ", func() {
		nic := ovirtsdk.Nic{}
		nic.SetId("NIC_id")
		nic.SetPlugged(true)
		nic.SetOnBoot(true)

		failures := validateNic(&nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicInterfaceCheckID))
	})
	It("should flag nic with on_boot == false: ", func() {
		var nic = newNic()
		nic.SetOnBoot(false)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicOnBootID))
	})
	It("should flag nic with plugged == false: ", func() {
		var nic = newNic()
		nic.SetPlugged(false)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicPluggedID))
	})
	It("should flag nic with port mirroring: ", func() {
		var nic = newNic()
		nic.MustVnicProfile().SetPortMirroring(true)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicVNicPortMirroringID))
	})
	It("should flag nic with pass-through == 'enabled': ", func() {
		var nic = newNic()
		passThrough := ovirtsdk.VnicPassThrough{}
		passThrough.SetMode(ovirtsdk.VnicPassThroughMode("enabled"))
		profile := nic.MustVnicProfile()
		profile.SetPassThrough(&passThrough)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicVNicPassThroughID))
	})
	It("should flag nic with some custom_properties ", func() {
		var nic = newNic()
		nic.MustVnicProfile()
		profile := nic.MustVnicProfile()

		property := ovirtsdk.CustomProperty{}
		property.SetName("property name")
		customPropertySlice := []*ovirtsdk.CustomProperty{&property}
		properties := ovirtsdk.CustomPropertySlice{}
		properties.SetSlice(customPropertySlice)
		profile.SetCustomProperties(&properties)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicVNicCustomPropertiesID))
	})
	It("should flag nic with network filter ", func() {
		var nic = newNic()
		nic.MustVnicProfile()
		profile := nic.MustVnicProfile()

		filter := ovirtsdk.NetworkFilter{}
		filter.SetId("nf ID")
		profile.SetNetworkFilter(&filter)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicVNicNetworkFilterID))
	})
	It("should flag nic with QOS ", func() {
		var nic = newNic()
		nic.MustVnicProfile()
		profile := nic.MustVnicProfile()

		qos := ovirtsdk.Qos{}
		qos.SetId("qos_id")
		profile.SetQos(&qos)

		failures := validateNic(nic)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NicVNicQosID))
	})
	It("should not flag two nics ", func() {
		nic1 := newNic()
		nic2 := newNic()
		nics := []*ovirtsdk.Nic{nic1, nic2}
		failures := ValidateNics(nics)

		Expect(failures).To(BeEmpty())
	})
	It("should flag two nics ", func() {
		nic1 := newNic()
		nic1.SetPlugged(false)
		nic2 := newNic()
		nic2.SetOnBoot(false)
		nics := []*ovirtsdk.Nic{nic1, nic2}
		failures := ValidateNics(nics)

		Expect(failures).To(HaveLen(2))

		var checkIDs = [2]CheckID{failures[0].ID, failures[1].ID}
		Expect(checkIDs).To(ContainElement(NicPluggedID))
		Expect(checkIDs).To(ContainElement(NicOnBootID))
	})
})

func newNic() *ovirtsdk.Nic {
	vnicProfile := ovirtsdk.VnicProfile{}
	vnicProfile.SetName("A Vnic Profile")

	nic := ovirtsdk.Nic{}
	nic.SetId(fmt.Sprintf("ID_%d", rand.Int()))
	nic.SetPlugged(true)
	nic.SetOnBoot(true)
	nic.SetVnicProfile(&vnicProfile)
	nic.SetInterface(ovirtsdk.NicInterface("virtio"))
	return &nic
}
