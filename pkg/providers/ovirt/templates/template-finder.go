package templates

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/os"

	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
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
func (f *TemplateFinder) FindTemplate(vm *ovirtsdk.Vm) (*templatev1.Template, error) {
	os, err := f.osFinder.FindOperatingSystem(vm)
	if err != nil {
		return nil, err
	}
	workload := getWorkload(vm)
	return f.getTemplate(os, workload)
}

func getWorkload(vm *ovirtsdk.Vm) string {
	// vm type is always available
	vmType, _ := vm.Type()
	// we need to remove underscore from high_performance, other workloads are OK
	return strings.Replace(string(vmType), "_", "", -1)
}

func (f *TemplateFinder) getTemplate(os string, workload string) (*templatev1.Template, error) {
	// We update metadata from the source vm so we default to medium flavor
	namespace := TemplateNamespace
	flavor := defaultFlavor
	templates, err := f.templateProvider.Find(&namespace, &os, &workload, &flavor)
	if err != nil {
		return nil, err
	}
	if len(templates.Items) == 0 {
		return nil, fmt.Errorf("Template not found for %s OS and %s workload", os, workload)
	}
	if len(templates.Items) > 1 {
		sort.Slice(templates.Items, func(i, j int) bool {
			return templates.Items[j].CreationTimestamp.Before(&templates.Items[i].CreationTimestamp)
		})
	}
	// Take first which matches label selector
	return &templates.Items[0], nil
}

// GetMetadata fetches OS and workload specific labels and annotations
func (f *TemplateFinder) GetMetadata(template *templatev1.Template, vm *ovirtsdk.Vm) (map[string]string, map[string]string, error) {
	os, err := f.osFinder.FindOperatingSystem(vm)
	if err != nil {
		return map[string]string{}, map[string]string{}, err
	}
	workload := getWorkload(vm)
	flavor := defaultFlavor
	labels := templates.OSLabelBuilder(&os, &workload, &flavor)

	key := fmt.Sprintf(templates.TemplateNameOsAnnotation, os)
	annotations := map[string]string{
		key: template.GetAnnotations()[key],
	}
	return labels, annotations, nil
}
