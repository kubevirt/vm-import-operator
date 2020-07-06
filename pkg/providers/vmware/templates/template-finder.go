package templates

import (
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/vmware/govmomi/vim25/mo"
)

const (
	// TemplateNamespace stores the default namespace for kubevirt templates
	TemplateNamespace = "openshift"
	defaultFlavor     = "medium"
)

// TemplateFinder attempts to find a template based on given parameters
type TemplateFinder struct {
	templateProvider templates.TemplateProvider
	osFinder         os.OSFinder
}

// NewTemplateFinder creates new TemplateFinder
func NewTemplateFinder(templateProvider templates.TemplateProvider, osFinder os.OSFinder) *TemplateFinder {
	return &TemplateFinder{
		templateProvider: templateProvider,
		osFinder:         osFinder,
	}
}

// FindTemplate attempts to find best match for a template based on the source VM
func (f *TemplateFinder) FindTemplate(vm *mo.VirtualMachine) (*templatev1.Template, error) {
	os, err := f.osFinder.FindOperatingSystem(vm)
	if err != nil {
		return nil, err
	}
	// We update metadata from the source vm so we default to medium flavor
	namespace := TemplateNamespace
	flavor := defaultFlavor
	tmpls, err := f.templateProvider.Find(&namespace, &os, nil, &flavor)
	if err != nil {
		return nil, err
	}
	if len(tmpls.Items) == 0 {
		return nil, fmt.Errorf("template not found for %s OS", os)
	}
	// Take first which matches label selector
	return &tmpls.Items[0], nil
}

// GetMetadata fetches OS and workload specific labels and annotations
func (f *TemplateFinder) GetMetadata(template *templatev1.Template, vm *mo.VirtualMachine) (map[string]string, map[string]string, error) {
	os, err := f.osFinder.FindOperatingSystem(vm)
	if err != nil {
		return map[string]string{}, map[string]string{}, err
	}
	flavor := defaultFlavor
	labels := templates.OSLabelBuilder(&os, nil, &flavor)

	key := fmt.Sprintf(templates.TemplateNameOsAnnotation, os)
	annotations := map[string]string{
		key: template.GetAnnotations()[key],
	}
	return labels, annotations, nil
}
