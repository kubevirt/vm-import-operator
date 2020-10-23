package client_test

import (
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
)

var _ = Describe("Test VMware rich client", func() {

	It("should fail to create a client if the scheme is invalid", func() {
		_, err := client.NewRichVMWareClient("invalidUrl", "username", "password", "")
		Expect(err).ToNot(BeNil())
	})

	It("should fail to create a client if it can't connect to the host", func() {
		model := simulator.VPX()
		_ = model.Create()
		server := model.Service.NewServer()
		unreachableApiUrl := server.URL.String()
		username := server.URL.User.Username()
		password, _ := server.URL.User.Password()
		server.Close()
		model.Remove()
		_, err := client.NewRichVMWareClient(unreachableApiUrl, username, password, "")
		Expect(err).ToNot(BeNil())
	})

	DescribeTable("should connect to the host", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		err = richClient.TestConnection()
		Expect(err).To(BeNil())
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should retrieve a VM by ID", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		vmRef, uuid := getVMIdentifiers()

		rawVm, err := richClient.GetVM(&uuid, nil, nil, nil)

		Expect(err).To(BeNil())
		retrievedVm, ok := rawVm.(*object.VirtualMachine)
		Expect(ok).To(BeTrue())
		Expect(retrievedVm.Reference().Value).To(Equal(vmRef))
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should return an error when a VM is not found by ID", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		uuid := "invalidUuid"

		_, err = richClient.GetVM(&uuid, nil, nil, nil)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vm 'invalidUuid' not found"))
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should retrieve a VM by name", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())

		rawVm, err := richClient.GetVM(nil, &vm.Name, nil, nil)

		Expect(err).To(BeNil())
		retrievedVm, ok := rawVm.(*object.VirtualMachine)
		Expect(ok).To(BeTrue())
		Expect(retrievedVm.Reference()).To(Equal(vm.Reference()))
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should return an error when a VM is not found by name", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		name := "invalidName"

		_, err = richClient.GetVM(nil, &name, nil, nil)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vm 'invalidName' not found"))
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should create a snapshot for a VM", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		moRef, _ := getVMIdentifiers()

		snapshotRef, err := richClient.CreateVMSnapshot(moRef, "name", "description", false, true)
		Expect(err).To(BeNil())
		Expect(snapshotRef).ToNot(BeNil())
		Expect(snapshotRef.Type).To(Equal("VirtualMachineSnapshot"))
		Expect(snapshotRef.Value[0:9]).To(Equal("snapshot-"))
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should power off and on a VM by ID", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		moRef, _ := getVMIdentifiers()

		err = richClient.StopVM(moRef)
		Expect(err).To(BeNil())

		err = richClient.StartVM(moRef)
		Expect(err).To(BeNil())
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should not throw an error when trying to power off an VM that's already off", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		moRef, _ := getVMIdentifiers()

		err = richClient.StopVM(moRef)
		Expect(err).To(BeNil())

		err = richClient.StopVM(moRef)
		Expect(err).To(BeNil())
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)

	DescribeTable("should not throw an error when trying to power on a VM that's already on", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		moRef, _ := getVMIdentifiers()

		err = richClient.StartVM(moRef)
		Expect(err).To(BeNil())

		err = richClient.StartVM(moRef)
		Expect(err).To(BeNil())
	},
		Entry("vCenter", simulator.VPX()),
		Entry("ESXi", simulator.ESX()),
	)
})

func createRichClient(server *simulator.Server) (*client.RichVmwareClient, error) {
	username := server.URL.User.Username()
	password, _ := server.URL.User.Password()
	return client.NewRichVMWareClient(server.URL.String(), username, password, "")
}

func getVMIdentifiers() (string, string) {
	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	return vm.Reference().Value, vm.Config.Uuid
}
