package validators_test

import (
	"fmt"
	"math/rand"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

var _ = Describe("Validating NIC", func() {
	table.DescribeTable("should flag nic with illegal interface model: ", func(iface string) {
		var nic = newNic()
		nic.SetInterface(ovirtsdk.NicInterface(iface))
		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicInterfaceCheckID))
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

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

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

		nics := []*ovirtsdk.Nic{&nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicInterfaceCheckID))
	})
	It("should flag nic with on_boot == false: ", func() {
		var nic = newNic()
		nic.SetOnBoot(false)

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicOnBootID))
	})
	It("should flag nic with plugged == false: ", func() {
		var nic = newNic()
		nic.SetPlugged(false)

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicPluggedID))
	})
	It("should flag nic with port mirroring: ", func() {
		var nic = newNic()
		nic.MustVnicProfile().SetPortMirroring(true)

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicVNicPortMirroringID))
	})
	It("should flag nic with pass-through == 'enabled': ", func() {
		var nic = newNic()
		passThrough := ovirtsdk.VnicPassThrough{}
		passThrough.SetMode(ovirtsdk.VnicPassThroughMode("enabled"))
		profile := nic.MustVnicProfile()
		profile.SetPassThrough(&passThrough)

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicVNicPassThroughID))
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

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicVNicCustomPropertiesID))
	})
	It("should flag nic with network filter ", func() {
		var nic = newNic()
		nic.MustVnicProfile()
		profile := nic.MustVnicProfile()

		filter := ovirtsdk.NetworkFilter{}
		filter.SetId("nf ID")
		profile.SetNetworkFilter(&filter)

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicVNicNetworkFilterID))
	})
	It("should flag nic with QOS ", func() {
		var nic = newNic()
		nic.MustVnicProfile()
		profile := nic.MustVnicProfile()

		qos := ovirtsdk.Qos{}
		qos.SetId("qos_id")
		profile.SetQos(&qos)

		nics := []*ovirtsdk.Nic{nic}

		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NicVNicQosID))
	})
	It("should not flag two nics ", func() {
		nic1 := newNic()
		nic2 := newNic()
		nics := []*ovirtsdk.Nic{nic1, nic2}
		failures := validators.ValidateNics(nics)

		Expect(failures).To(BeEmpty())
	})
	It("should flag two nics ", func() {
		nic1 := newNic()
		nic1.SetPlugged(false)
		nic2 := newNic()
		nic2.SetOnBoot(false)
		nics := []*ovirtsdk.Nic{nic1, nic2}
		failures := validators.ValidateNics(nics)

		Expect(failures).To(HaveLen(2))

		var checkIDs = [2]validators.CheckID{failures[0].ID, failures[1].ID}
		Expect(checkIDs).To(ContainElement(validators.NicPluggedID))
		Expect(checkIDs).To(ContainElement(validators.NicOnBootID))
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
