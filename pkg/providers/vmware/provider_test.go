package vmware

import (
	"context"
	"encoding/json"
	"github.com/onsi/ginkgo/extensions/table"

	"github.com/ghodss/yaml"
	"github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	providers "github.com/kubevirt/vm-import-operator/pkg/providers"
	vclient "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/client"
	vtemplates "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/templates"
	"github.com/kubevirt/vm-import-operator/pkg/templates"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

var (
	poweredOffUUID = "265104de-1472-547c-b873-6dc7883fb6cb"
	poweredOnUUID  = "39365506-5a0a-5fd0-be10-9586ad53aaad"
)

func makeProvider() (*simulator.Model, *simulator.Server, *VmwareProvider) {
	model := simulator.VPX()
	_ = model.Create()
	server := model.Service.NewServer()
	username := server.URL.User.Username()
	password, _ := server.URL.User.Password()
	vmwareClient, err := vclient.NewRichVMWareClient(server.URL.String(), username, password, "")
	Expect(err).To(BeNil())
	provider := &VmwareProvider{
		vmwareClient: vmwareClient,
		instance: &v1beta1.VirtualMachineImport{
			Spec: v1beta1.VirtualMachineImportSpec{},
		},
	}
	return model, server, provider
}

func makeSecret(apiUrl, username, password, thumbprint *string) *v1.Secret {
	secretData := make(map[string]string)
	if apiUrl != nil {
		secretData["apiUrl"] = *apiUrl
	}
	if username != nil {
		secretData["username"] = *username
	}
	if password != nil {
		secretData["password"] = *password
	}
	if thumbprint != nil {
		secretData["thumbprint"] = *thumbprint
	}
	encoded, _ := yaml.Marshal(secretData)

	return &v1.Secret{
		Data: map[string][]byte{
			"vmware": encoded,
		},
	}
}

func getSimulatorVM() *simulator.VirtualMachine {
	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	return vm
}

func getSimulatorVMIdentifiers(vm *simulator.VirtualMachine) (string, string, string) {
	return vm.Reference().Value, vm.Config.Uuid, vm.Name
}

var _ = Describe("Initialization", func() {
	provider := VmwareProvider{}
	instance := &v1beta1.VirtualMachineImport{}
	apiUrl := "https://my.vsphere.example/sdk"
	username := "user"
	password := "pass"
	thumbprint := "thumb"
	empty := ""

	It("should initialize succesfully when all required fields are present", func() {
		secret := makeSecret(&apiUrl, &username, &password, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).To(BeNil())
	})

	It("should initialize succesfully when the thumbprint is missing", func() {
		secret := makeSecret(&apiUrl, &username, &password, nil)
		err := provider.Init(secret, instance)
		Expect(err).To(BeNil())
	})

	It("should initialize succesfully when the thumbprint is empty", func() {
		secret := makeSecret(&apiUrl, &username, &password, &empty)
		err := provider.Init(secret, instance)
		Expect(err).To(BeNil())
	})

	It("should fail to initialize when the apiUrl is missing", func() {
		secret := makeSecret(nil, &username, &password, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vmware secret must contain apiUrl attribute"))
	})

	It("should fail to initialize when the apiUrl is empty", func() {
		secret := makeSecret(&empty, &username, &password, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vmware secret apiUrl cannot be empty"))
	})

	It("should fail to initialize when the username is missing", func() {
		secret := makeSecret(&apiUrl, nil, &password, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vmware secret must contain username attribute"))
	})

	It("should fail to initialize when the username is empty", func() {
		secret := makeSecret(&apiUrl, &empty, &password, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vmware secret username cannot be empty"))
	})

	It("should fail to initialize when the password is missing", func() {
		secret := makeSecret(&apiUrl, &username, nil, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vmware secret must contain password attribute"))
	})

	It("should fail to initialize when the password is empty", func() {
		secret := makeSecret(&apiUrl, &username, &empty, &thumbprint)
		err := provider.Init(secret, instance)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(Equal("vmware secret password cannot be empty"))
	})
})

var _ = Describe("LoadVM", func() {

	var provider *VmwareProvider
	var model *simulator.Model
	var server *simulator.Server

	BeforeEach(func() {
		model, server, provider = makeProvider()
	})

	AfterEach(func() {
		server.Close()
		model.Remove()
	})

	It("Should load a VM by UUID", func() {
		vm := getSimulatorVM()
		moRef, uuid, _ := getSimulatorVMIdentifiers(vm)
		sourceSpec := v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   &uuid,
					Name: nil,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		err := provider.LoadVM(sourceSpec)
		Expect(err).To(BeNil())
		Expect(provider.vm).ToNot(BeNil())
		Expect(provider.vm.Reference().Value).To(Equal(moRef))
		Expect(provider.vm.UUID(context.Background())).To(Equal(uuid))
	})

	It("Should load a VM by name", func() {
		vm := getSimulatorVM()
		moRef, uuid, name := getSimulatorVMIdentifiers(vm)
		sourceSpec := v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   nil,
					Name: &name,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		err := provider.LoadVM(sourceSpec)
		Expect(err).To(BeNil())
		Expect(provider.vm).ToNot(BeNil())
		Expect(provider.vm.Reference().Value).To(Equal(moRef))
		Expect(provider.vm.UUID(context.Background())).To(Equal(uuid))
	})
})

var _ = Describe("GetVMName", func() {
	var provider *VmwareProvider
	var model *simulator.Model
	var server *simulator.Server

	BeforeEach(func() {
		model, server, provider = makeProvider()
	})

	AfterEach(func() {
		server.Close()
		model.Remove()
	})

	It("Should get the name of a VM that is identified by UUID", func() {
		vm := getSimulatorVM()
		_, uuid, expectedName := getSimulatorVMIdentifiers(vm)
		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   &uuid,
					Name: nil,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		retrievedName, err := provider.GetVMName()
		Expect(err).To(BeNil())
		Expect(provider.vm).ToNot(BeNil())
		Expect(retrievedName).To(Equal(expectedName))
	})

	It("Should get the name of a VM that is identified by Name", func() {
		vm := getSimulatorVM()
		_, _, expectedName := getSimulatorVMIdentifiers(vm)
		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   nil,
					Name: &expectedName,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		retrievedName, err := provider.GetVMName()
		Expect(err).To(BeNil())
		Expect(provider.vm).ToNot(BeNil())
		Expect(retrievedName).To(Equal(expectedName))
	})
})

var _ = Describe("GetVMStatus", func() {
	var provider *VmwareProvider
	var model *simulator.Model
	var server *simulator.Server

	BeforeEach(func() {
		model, server, provider = makeProvider()
	})

	AfterEach(func() {
		server.Close()
		model.Remove()
	})

	It("Should get the status of a powered off VM identified by UUID", func() {
		vm := getSimulatorVM()
		_, uuid, _ := getSimulatorVMIdentifiers(vm)
		vm.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOff
		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   &uuid,
					Name: nil,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusDown))
	})

	It("Should get the status of a powered off VM identified by Name", func() {
		vm := getSimulatorVM()
		_, _, name := getSimulatorVMIdentifiers(vm)
		vm.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOff
		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   nil,
					Name: &name,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusDown))
	})

	It("Should get the status of a powered on VM identified by UUID", func() {
		vm := getSimulatorVM()
		_, uuid, _ := getSimulatorVMIdentifiers(vm)
		vm.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   &uuid,
					Name: nil,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusUp))
	})

	It("Should get the status of a powered off VM identified by Name", func() {
		vm := getSimulatorVM()
		_, _, name := getSimulatorVMIdentifiers(vm)
		vm.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   nil,
					Name: &name,
				},
			},
		}

		Expect(provider.vm).To(BeNil())
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusUp))
	})
})

var _ = Describe("StartVM and StopVM", func() {
	var provider VmwareProvider
	var model *simulator.Model
	var server *simulator.Server
	mockClient := &mockClient{}

	BeforeEach(func() {
		model = simulator.VPX()
		_ = model.Load("../../../tests/vmware/vcsim")
		server = model.Service.NewServer()
		username := server.URL.User.Username()
		password, _ := server.URL.User.Password()
		vmwareClient, err := vclient.NewRichVMWareClient(server.URL.String(), username, password, "")
		Expect(err).To(BeNil())
		provider = VmwareProvider{
			vmwareClient: vmwareClient,
			instance: &v1beta1.VirtualMachineImport{
				Spec: v1beta1.VirtualMachineImportSpec{},
			},
		}

		provider.instance.Spec.Source = v1beta1.VirtualMachineImportSourceSpec{
			Vmware: &v1beta1.VirtualMachineImportVmwareSourceSpec{
				VM: v1beta1.VirtualMachineImportVmwareSourceVMSpec{
					ID:   nil,
					Name: nil,
				},
			},
		}
	})

	AfterEach(func() {
		server.Close()
		model.Remove()
	})

	It("Should launch an async power on task without an error", func() {
		provider.instance.Spec.Source.Vmware.VM.ID = &poweredOffUUID
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusDown))

		err = provider.StartVM()
		Expect(err).To(BeNil())
	})

	It("Should not error if the VM is already powered on", func() {
		provider.instance.Spec.Source.Vmware.VM.ID = &poweredOnUUID
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusUp))

		err = provider.StartVM()
		Expect(err).To(BeNil())
	})

	It("Should power off a VM without an error", func() {
		provider.instance.Spec.Source.Vmware.VM.ID = &poweredOnUUID
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusUp))

		err = provider.StopVM(provider.instance, mockClient)
		Expect(err).To(BeNil())

		provider.vmProperties = nil
		powerState, err = provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusDown))
	})

	It("Should not error if the VM is already powered off", func() {
		provider.instance.Spec.Source.Vmware.VM.ID = &poweredOffUUID
		powerState, err := provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusDown))

		err = provider.StopVM(provider.instance, mockClient)
		Expect(err).To(BeNil())

		provider.vmProperties = nil
		powerState, err = provider.GetVMStatus()
		Expect(err).To(BeNil())
		Expect(powerState).To(Equal(providers.VMStatusDown))
	})
})

var _ = Describe("TestConnection", func() {
	var provider *VmwareProvider
	var model *simulator.Model
	var server *simulator.Server

	BeforeEach(func() {
		model, server, provider = makeProvider()
	})

	It("should not return an error if the connection to the provider is good", func() {
		err := provider.TestConnection()
		server.Close()
		model.Remove()
		Expect(err).To(BeNil())
	})

	It("should return an error if the connection to the provider is bad", func() {
		// this turns off the server, making it unreachable
		server.Close()
		model.Remove()
		err := provider.TestConnection()
		Expect(err).ToNot(BeNil())
	})
})

var _ = table.DescribeTable("Processing a template", func(workloadLabel string) {

	provider := VmwareProvider{}
	model := simulator.VPX()
	_ = model.Create()
	server := model.Service.NewServer()
	client, _ := govmomi.NewClient(context.TODO(), server.URL, false)

	simVm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	simVm.Guest.GuestId = "rhel7Guest"
	simulator.Map.Put(simVm)

	username := server.URL.User.Username()
	password, _ := server.URL.User.Password()
	richClient, _ := vclient.NewRichVMWareClient(server.URL.String(), username, password, "")
	provider.vmwareClient = richClient
	provider.vm = object.NewVirtualMachine(client.Client, simVm.Reference())
	templateProvider := &mockTemplateProvider{}
	provider.templateHandler = templates.NewTemplateHandler(templateProvider)
	provider.templateFinder = vtemplates.NewTemplateFinder(templateProvider, &mockOsFinder{})

	vmName := "test"
	namespace := "default"
	template := templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-template-name",
			Namespace: "kubevirt-hyperconverged",
			Annotations: map[string]string{
				"name.os.template.kubevirt.io/rhel7.7": "Red Hat Enterprise Linux 7.7",
			},
			Labels: map[string]string{
				"os.template.kubevirt.io/rhel7.7":    "true",
				"flavor.template.kubevirt.io/medium": "true",
				workloadLabel:                        "true",
			},
		},
	}

	vm, err := provider.ProcessTemplate(&template, &vmName, namespace)
	Expect(err).To(BeNil())
	vmLabels := map[string]string{
		workloadLabel:                        "true",
		"flavor.template.kubevirt.io/medium": "true",
		"os.template.kubevirt.io/rhel7.7":    "true",
		"app":                                "test",
		"vm.kubevirt.io/template":            "my-template-name",
		"vm.kubevirt.io/template.namespace":  "kubevirt-hyperconverged",
		"vm.kubevirt.io/template.revision":   "1",
		"vm.kubevirt.io/template.version":    "v0.10.0",
	}
	specLabels := map[string]string{
		workloadLabel:                        "true",
		"flavor.template.kubevirt.io/medium": "true",
		"kubevirt.io/domain":                 "test",
		"kubevirt.io/size":                   "medium",
		"os.template.kubevirt.io/rhel7.7":    "true",
		"vm.kubevirt.io/name":                "test",
	}
	annotations := map[string]string{
		"name.os.template.kubevirt.io/rhel7.7": "Red Hat Enterprise Linux 7.7",
	}
	Expect(vm.ObjectMeta.GetLabels()).Should(Equal(vmLabels))
	Expect(vm.Spec.Template.ObjectMeta.GetLabels()).Should(Equal(specLabels))
	Expect(vm.ObjectMeta.GetAnnotations()).Should(Equal(annotations))

},
	table.Entry("Desktop", "workload.template.kubevirt.io/desktop"),
	table.Entry("Server", "workload.template.kubevirt.io/server"),
)

type mockOsFinder struct{}

func (r *mockOsFinder) FindOperatingSystem(_ *mo.VirtualMachine) (string, error) {
	return "rhel7.7", nil
}

type mockTemplateProvider struct{}

func (t *mockTemplateProvider) Find(_ *string, _ *string, _ *string, _ *string) (*templatev1.TemplateList, error) {
	return nil, nil
}

func (t *mockTemplateProvider) Process(_ string, _ *string, _ *templatev1.Template) (*templatev1.Template, error) {
	vm := kubevirtv1.VirtualMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VirtualMachine",
			APIVersion: "kubevirt.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app":                               "test",
				"vm.kubevirt.io/template":           "my-template-name",
				"vm.kubevirt.io/template.namespace": "kubevirt-hyperconverged",
				"vm.kubevirt.io/template.revision":  "1",
				"vm.kubevirt.io/template.version":   "v0.10.0",
			},
			Name: "test",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"kubevirt.io/domain":                 "test",
						"kubevirt.io/size":                   "medium",
						"os.template.kubevirt.io/rhel7.7":    "true",
						"flavor.template.kubevirt.io/medium": "true",
						"vm.kubevirt.io/name":                "test",
					},
				},
			},
		},
	}
	rawBytes, _ := json.Marshal(vm)
	tmpl := &templatev1.Template{
		Objects: []runtime.RawExtension{
			{
				Raw: rawBytes,
			},
		},
	}
	return tmpl, nil
}

type mockClient struct{}

// Create implements client.Client
func (c *mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return nil
}

// Update implements client.Client
func (c *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

// Delete implements client.Client
func (c *mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return nil
}

// DeleteAllOf implements client.Client
func (c *mockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

// Patch implements client.Client
func (c *mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

// Get implements client.Client
func (c mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return nil
}

// List implements client.Client
func (c *mockClient) List(ctx context.Context, objectList runtime.Object, opts ...client.ListOption) error {
	return nil
}

// Status implements client.StatusClient
func (c *mockClient) Status() client.StatusWriter {
	return c
}
