package templates_test

import (
	"fmt"
	"time"

	"github.com/vmware/govmomi/vim25/mo"

	vtemplates "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/templates"
	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	findTemplatesMock func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error)
	findOs            func(vm *mo.VirtualMachine) (string, error)
)
var _ = Describe("Finding a Template", func() {
	templateFinder := vtemplates.NewTemplateFinder(&mockTemplateProvider{}, &mockOsFinder{})

	BeforeEach(func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			template := createTemplate(name, os, workload, flavor)
			templateList := createTemplatesList(template)
			return templateList, nil
		}
		findOs = func(vm *mo.VirtualMachine) (string, error) {
			return "linux", nil
		}
	})
	It("should find a template for given OS: ", func() {
		vm := &mo.VirtualMachine{}
		template, err := templateFinder.FindTemplate(vm)

		Expect(err).To(BeNil())
		Expect(template).To(Not(BeNil()))
	})
	It("should return a single template if when there is a multiple match: ", func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			template1 := createTemplate(name, os, workload, flavor)
			template2 := createTemplate(name, os, workload, flavor)
			templateList := createTemplatesList(template1, template2)
			return templateList, nil
		}

		vm := &mo.VirtualMachine{}
		template, err := templateFinder.FindTemplate(vm)

		Expect(err).To(BeNil())
		Expect(template).To(Not(BeNil()))
	})
	It("should fail to find a template: ", func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			return nil, fmt.Errorf("boom")
		}
		template, err := templateFinder.FindTemplate(&mo.VirtualMachine{})

		Expect(err).To(Not(BeNil()))
		Expect(template).To(BeNil())
	})
	It("should find newer template:", func() {
		now := metav1.Now()
		newer := metav1.NewTime(now.Add(time.Duration(1 * time.Minute)))
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			template1 := createTemplate(name, os, workload, flavor)
			template1.CreationTimestamp = now

			template2 := createTemplate(name, os, workload, flavor)
			template2.CreationTimestamp = newer

			templateList := createTemplatesList(template1, template2)
			return templateList, nil
		}

		vm := &mo.VirtualMachine{}
		template, err := templateFinder.FindTemplate(vm)

		Expect(err).To(BeNil())
		Expect(template.CreationTimestamp).To(Equal(newer))
	})
	It("should prefer a server template if one exists:", func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			template := createTemplate(name, os, workload, flavor)
			return createTemplatesList(template), nil
		}

		vm := &mo.VirtualMachine{}
		template, err := templateFinder.FindTemplate(vm)
		Expect(err).To(BeNil())
		Expect(template).ToNot(BeNil())
		Expect(template.Labels[fmt.Sprintf(templates.TemplateWorkloadLabel, "server")]).To(Equal("true"))
	})
	It("should fall back to finding a desktop template:", func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			if *workload == "server" {
				return createTemplatesList(), nil
			} else {
				template := createTemplate(name, os, workload, flavor)
				return createTemplatesList(template), nil
			}
		}

		vm := &mo.VirtualMachine{}
		template, err := templateFinder.FindTemplate(vm)
		Expect(err).To(BeNil())
		Expect(template).ToNot(BeNil())
		Expect(template.Labels[fmt.Sprintf(templates.TemplateWorkloadLabel, "desktop")]).To(Equal("true"))
	})
})

func createTemplate(name *string, os *string, workload *string, flavor *string) *templatev1.Template {
	template := templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *name,
			Namespace: "testns",
			Labels:    templates.OSLabelBuilder(os, workload, flavor),
		},
	}
	return &template
}

func createTemplatesList(templates ...*templatev1.Template) *templatev1.TemplateList {
	templateItems := make([]templatev1.Template, len(templates))
	for i, t := range templates {
		templateItems[i] = *t
	}
	templateList := templatev1.TemplateList{}
	templateList.Items = templateItems
	return &templateList
}

type mockTemplateProvider struct{}

// Find mocks the behavior of the client for calling template API to find template by labels
func (t *mockTemplateProvider) Find(
	name *string,
	os *string,
	workload *string,
	flavor *string,
) (*templatev1.TemplateList, error) {
	// namespace is assumed to be always 'openshift'
	return findTemplatesMock(name, os, workload, flavor)
}

// Process mocks the behavior of the client for calling process API
func (t *mockTemplateProvider) Process(namespace string, vmName *string, template *templatev1.Template) (*templatev1.Template, error) {
	return &templatev1.Template{}, nil
}

type mockOsFinder struct{}

func (o *mockOsFinder) FindOperatingSystem(vm *mo.VirtualMachine) (string, error) {
	return findOs(vm)
}
