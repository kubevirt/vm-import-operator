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

	DescribeTable("should power off and on a VM by ID", func(model *simulator.Model) {
		_ = model.Create()
		server := model.Service.NewServer()
		defer model.Remove()
		defer server.Close()
		richClient, err := createRichClient(server)
		Expect(err).To(BeNil())
		_, uuid := getVMIdentifiers()

		err = richClient.StopVM(uuid)
		Expect(err).To(BeNil())

		err = richClient.StartVM(uuid)
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
		_, uuid := getVMIdentifiers()

		err = richClient.StopVM(uuid)
		Expect(err).To(BeNil())

		err = richClient.StopVM(uuid)
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
		_, uuid := getVMIdentifiers()

		err = richClient.StartVM(uuid)
		Expect(err).To(BeNil())

		err = richClient.StartVM(uuid)
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