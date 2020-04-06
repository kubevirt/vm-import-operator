package templates_test

import (
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/templates"
	templatev1 "github.com/openshift/api/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	findTemplatesMock func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error)
)
var _ = Describe("Finding a Template", func() {
	templateFinder := templates.NewTemplateFinder(&mockTemplateProvider{})
	BeforeEach(func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			template := createTemplate(name, os, workload, flavor)
			templateList := createTemplatesList(template)
			return templateList, nil
		}
	})
	It("should find a template for given OS: ", func() {
		vmOS := ovirtsdk.OperatingSystem{}
		vmOS.SetType("RHEL")
		vm := ovirtsdk.NewVmBuilder().Os(&vmOS).MustBuild()
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
		template, err := templateFinder.FindTemplate(&ovirtsdk.Vm{})

		Expect(err).To(BeNil())
		Expect(template).To(Not(BeNil()))
	})
	It("should fail to find a template: ", func() {
		findTemplatesMock = func(name *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error) {
			return nil, fmt.Errorf("boom")
		}
		template, err := templateFinder.FindTemplate(&ovirtsdk.Vm{})

		Expect(err).To(Not(BeNil()))
		Expect(template).To(BeNil())
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
