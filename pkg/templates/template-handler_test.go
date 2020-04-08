package templates_test

import (
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	processTemplateMock func(namespace string, vmName string, template *templatev1.Template) (*templatev1.Template, error)
)
var _ = Describe("Processing a template", func() {
	templateFinder := templates.NewTemplateHandler(&mockTemplateProvider{})

	It("should process a template: ", func() {
		processTemplateMock = func(namespace string, vmName string, template *templatev1.Template) (*templatev1.Template, error) {
			objects := []runtime.RawExtension{}
			encoder := v1.Codecs.LegacyCodec(v1.GroupVersion)
			raw, _ := runtime.Encode(encoder, createVM(namespace, vmName))

			result := templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vmName,
					Namespace: namespace,
					Labels:    template.Labels,
				},
				Objects: append(objects, runtime.RawExtension{
					Raw: raw,
				}),
			}
			return &result, nil
		}
		vmName := "testName"
		os := "centos8"
		workload := "server"
		flavor := "medium"
		vm, err := templateFinder.ProcessTemplate(createTemplate(&vmName, &os, &workload, &flavor), vmName)

		Expect(vm.GetName()).To(Equal(vmName))
		Expect(vm.Spec.Template.Spec.Volumes).To(BeEmpty())
		Expect(vm.Spec.Template.Spec.Networks).To(BeEmpty())
		Expect(vm.Spec.DataVolumeTemplates).To(BeEmpty())
		Expect(err).To(BeNil())
	})
	It("should fail to process a template: ", func() {
		processTemplateMock = func(namespace string, vmName string, template *templatev1.Template) (*templatev1.Template, error) {
			return nil, fmt.Errorf("oh my!")
		}
		vm, err := templateFinder.ProcessTemplate(&templatev1.Template{}, "")

		Expect(vm).To(BeNil())
		Expect(err).To(Not(BeNil()))
	})
})

func createTemplate(name *string, os *string, workload *string, flavor *string) *templatev1.Template {
	template := templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *name,
			Namespace: templates.TemplateNamespace,
			Labels:    templates.OSLabelBuilder(os, workload, flavor),
		},
	}
	return &template
}

type mockTemplateProvider struct{}

// Find mocks the behavior of the client for calling template API to find template by labels
func (t *mockTemplateProvider) Find(
	name *string,
	os *string,
	workload *string,
	flavor *string,
) (*templatev1.TemplateList, error) {
	return &templatev1.TemplateList{}, nil
}

func createVM(namespace string, name string) *v1.VirtualMachine {
	labels := map[string]string{"name": name}
	running := false
	memory := resource.NewQuantity(128, resource.DecimalSI)

	return &v1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.VirtualMachineSpec{
			Running: &running,
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    labels,
					Name:      name,
					Namespace: namespace,
				},
				Spec: v1.VirtualMachineInstanceSpec{
					Volumes:  createVolumes(),
					Networks: createNetworks(),
					Domain: v1.DomainSpec{
						Memory: &v1.Memory{
							Guest: memory,
						},
					},
				},
			},
			DataVolumeTemplates: createDataVolumes(namespace, name),
		},
	}
}

func createDataVolumes(namespace string, name string) []cdiv1.DataVolume {
	dvs := []cdiv1.DataVolume{}
	return append(dvs, cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
}

func createVolumes() []v1.Volume {
	vls := []v1.Volume{}
	return append(vls, v1.Volume{
		Name: "testVolume",
		VolumeSource: v1.VolumeSource{
			ContainerDisk: &v1.ContainerDiskSource{
				Image: "kubevirt/cirros-container-disk-demo",
			},
		},
	})
}

func createNetworks() []v1.Network {
	nets := []v1.Network{}
	return append(nets, v1.Network{
		Name: "testNetwork",
		NetworkSource: v1.NetworkSource{
			Pod: &v1.PodNetwork{},
		},
	})
}

// Process mocks the behavior of the client for calling process API
func (t *mockTemplateProvider) Process(namespace string, vmName string, template *templatev1.Template) (*templatev1.Template, error) {
	return processTemplateMock(namespace, vmName, template)
}
