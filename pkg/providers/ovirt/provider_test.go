package ovirtprovider

import (
	"encoding/json"

	otemplates "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/templates"
	templates "github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/client-go/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	process func(namespace string, vmName *string, template *templatev1.Template) (*templatev1.Template, error)
)

var _ = Describe("Processing a template", func() {
	provider := OvirtProvider{}

	BeforeEach(func() {
		provider.vm = &ovirtsdk.Vm{}
		os, _ := ovirtsdk.NewOperatingSystemBuilder().Type("windows_2012R2x64").Build()
		provider.vm.SetOs(os)
		provider.vm.SetType("server")

		templateProvider := &mockTemplateProvider{}
		provider.templateHandler = templates.NewTemplateHandler(templateProvider)
		provider.templateFinder = otemplates.NewTemplateFinder(templateProvider, &mockOSMapProvider{})

		process = func(namespace string, vmName *string, temp *templatev1.Template) (*templatev1.Template, error) {
			vm := kubevirtv1.VirtualMachine{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualMachine",
					APIVersion: "kubevirt.io/v1alpha3",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                               "test",
						"vm.kubevirt.io/template":           "win2k12r2-server-medium",
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
								"kubevirt.io/domain":                   "test",
								"kubevirt.io/size":                     "medium",
								"os.template.kubevirt.io/win2k12r2":    "true",
								"workload.template.kubevirt.io/server": "true",
								"flavor.template.kubevirt.io/medium":   "true",
								"vm.kubevirt.io/name":                  "test",
							},
						},
					},
				},
			}
			rawBytes, _ := json.Marshal(vm)
			template := templatev1.Template{
				Objects: []runtime.RawExtension{
					runtime.RawExtension{
						Raw: rawBytes,
					},
				},
			}
			return &template, nil
		}
	})

	It("should process a template: ", func() {
		vmName := "test"
		namespace := "default"
		template := templatev1.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "win2k12r2-server-medium-v0.7.0",
				Namespace: "kubevirt-hyperconverged",
				Annotations: map[string]string{
					"name.os.template.kubevirt.io/win2k12r2": "Microsoft Windows Server 2012 R2",
				},
			},
		}

		vm, err := provider.ProcessTemplate(&template, &vmName, namespace)

		Expect(err).To(BeNil())
		vmLabels := map[string]string{
			"flavor.template.kubevirt.io/medium":   "true",
			"os.template.kubevirt.io/win2k12r2":    "true",
			"workload.template.kubevirt.io/server": "true",
			"app":                                  "test",
			"vm.kubevirt.io/template":              "win2k12r2-server-medium-v0.7.0",
			"vm.kubevirt.io/template.namespace":    "kubevirt-hyperconverged",
			"vm.kubevirt.io/template.revision":     "1",
			"vm.kubevirt.io/template.version":      "v0.10.0",
		}
		specLabels := map[string]string{
			"workload.template.kubevirt.io/server": "true",
			"flavor.template.kubevirt.io/medium":   "true",
			"kubevirt.io/domain":                   "test",
			"kubevirt.io/size":                     "medium",
			"os.template.kubevirt.io/win2k12r2":    "true",
			"vm.kubevirt.io/name":                  "test",
		}
		annotations := map[string]string{
			"name.os.template.kubevirt.io/win2k12r2": "Microsoft Windows Server 2012 R2",
		}
		Expect(vm.ObjectMeta.GetLabels()).Should(Equal(vmLabels))
		Expect(vm.Spec.Template.ObjectMeta.GetLabels()).Should(Equal(specLabels))
		Expect(vm.ObjectMeta.GetAnnotations()).Should(Equal(annotations))
	})
})

type mockTemplateProvider struct{}

type mockOSMapProvider struct{}

func (t *mockTemplateProvider) Find(namespace *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
	return nil, nil
}

func (t *mockTemplateProvider) Process(namespace string, vmName *string, template *templatev1.Template) (*templatev1.Template, error) {
	return process(namespace, vmName, template)
}

func (os *mockOSMapProvider) GetOSMaps() (map[string]string, map[string]string, error) {
	return map[string]string{}, map[string]string{
		"windows_2012R2x64": "win2k12r2",
	}, nil
}
