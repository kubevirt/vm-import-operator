package client_test

import (
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
)

var _ = Describe("Test VMware rich client with vCenter", func() {
	var (
		model               *simulator.Model
		server              *simulator.Server
		vm                  *simulator.VirtualMachine
		username            string
		password            string
		simulatorThumbprint = ""
	)

	BeforeEach(func() {
		model = simulator.VPX()
		_ = model.Create()
		server = model.Service.NewServer()
		vm = simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		username = server.URL.User.Username()
		password, _ = server.URL.User.Password()
	})

	AfterEach(func() {
		model.Remove()
		server.Close()
	})

	It("should connect to vCenter", func() {
		_, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
	})

	It("should retrieve a VM by ID", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		rawVm, err := richClient.GetVM(&vmRef, nil, nil, nil)
		Expect(err).To(BeNil())
		retrievedVm, ok := rawVm.(*object.VirtualMachine)
		Expect(ok).To(BeTrue())
		Expect(retrievedVm.Reference()).To(Equal(vm.Reference()))
	})

	It("should power off and on a VM by ID", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StartVM(vmRef)
		Expect(err).To(BeNil())
	})

	It("should not throw an error when trying to power off an VM that's already off", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
	})

	It("should not throw an error when trying to power on a VM that's already on", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StartVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StartVM(vmRef)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("Test VMware rich client with ESXi", func() {
	var (
		model               *simulator.Model
		server              *simulator.Server
		vm                  *simulator.VirtualMachine
		username            string
		password            string
		simulatorThumbprint = ""
	)

	BeforeEach(func() {
		model = simulator.ESX()
		_ = model.Create()
		server = model.Service.NewServer()
		vm = simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
		username = server.URL.User.Username()
		password, _ = server.URL.User.Password()
	})

	AfterEach(func() {
		model.Remove()
		server.Close()
	})

	It("should connect to the ESXi host", func() {
		_, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
	})

	It("should retrieve a VM by ID", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		rawVm, err := richClient.GetVM(&vmRef, nil, nil, nil)
		Expect(err).To(BeNil())
		retrievedVm, ok := rawVm.(*object.VirtualMachine)
		Expect(ok).To(BeTrue())
		Expect(retrievedVm.Reference()).To(Equal(vm.Reference()))
	})

	It("should power off and on a VM by ID", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StartVM(vmRef)
		Expect(err).To(BeNil())
	})

	It("should not throw an error when trying to power off an VM that's already off", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
	})

	It("should not throw an error when trying to power on a VM that's already on", func() {
		richClient, err := client.NewRichVMWareClient(server.URL.String(), username, password, simulatorThumbprint)
		Expect(err).To(BeNil())
		vmRef := vm.Reference().Value
		err = richClient.StopVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StartVM(vmRef)
		Expect(err).To(BeNil())
		err = richClient.StartVM(vmRef)
		Expect(err).To(BeNil())
	})
})
