package vmware

import (
	"context"
	"encoding/json"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("Processing a template", func() {
	provider := VmwareProvider{}

	BeforeEach(func() {
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
	})

	It("should process a template", func() {
		vmName := "test"
		namespace := "default"
		template := templatev1.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhel7.7-server-medium",
				Namespace: "kubevirt-hyperconverged",
				Annotations: map[string]string{
					"name.os.template.kubevirt.io/rhel7.7": "Red Hat Enterprise Linux 7.7",
				},
			},
		}

		vm, err := provider.ProcessTemplate(&template, &vmName, namespace)
		Expect(err).To(BeNil())
		vmLabels := map[string]string{
			"flavor.template.kubevirt.io/medium":   "true",
			"os.template.kubevirt.io/rhel7.7":      "true",
			"workload.template.kubevirt.io/server": "true",
			"app":                                  "test",
			"vm.kubevirt.io/template":              "rhel7.7-server-medium",
			"vm.kubevirt.io/template.namespace":    "kubevirt-hyperconverged",
			"vm.kubevirt.io/template.revision":     "1",
			"vm.kubevirt.io/template.version":      "v0.10.0",
		}
		specLabels := map[string]string{
			"workload.template.kubevirt.io/server": "true",
			"flavor.template.kubevirt.io/medium":   "true",
			"kubevirt.io/domain":                   "test",
			"kubevirt.io/size":                     "medium",
			"os.template.kubevirt.io/rhel7.7":      "true",
			"vm.kubevirt.io/name":                  "test",
		}
		annotations := map[string]string{
			"name.os.template.kubevirt.io/rhel7.7": "Red Hat Enterprise Linux 7.7",
		}
		Expect(vm.ObjectMeta.GetLabels()).Should(Equal(vmLabels))
		Expect(vm.Spec.Template.ObjectMeta.GetLabels()).Should(Equal(specLabels))
		Expect(vm.ObjectMeta.GetAnnotations()).Should(Equal(annotations))
	})
})

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
				"vm.kubevirt.io/template":           "rhel7.7-server-medium",
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
