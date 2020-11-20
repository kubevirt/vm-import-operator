package templates

import (
	"fmt"
	"sort"

	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/vmware/govmomi/vim25/mo"
)

const (
	// TemplateNamespace stores the default namespace for kubevirt templates
	templateNamespace = "openshift"
	defaultFlavor     = "medium"
)

var (
	serverWorkload  = "server"
	desktopWorkload = "desktop"
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
	namespace := templateNamespace
	flavor := defaultFlavor
	tmpls, err := f.templateProvider.Find(&namespace, &os, &serverWorkload, &flavor)
	if err != nil {
		return nil, err
	}

	// no server template was found, look for a desktop template instead.
	if len(tmpls.Items) == 0 {
		tmpls, err = f.templateProvider.Find(&namespace, &os, &desktopWorkload, &flavor)
		if err != nil {
			return nil, err
		}
		if len(tmpls.Items) == 0 {
			return nil, fmt.Errorf("template not found for %s OS", os)
		}
	}

	if len(tmpls.Items) > 1 {
		sort.Slice(tmpls.Items, func(i, j int) bool {
			return tmpls.Items[j].CreationTimestamp.Before(&tmpls.Items[i].CreationTimestamp)
		})
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
	key := fmt.Sprintf(templates.TemplateNameOsAnnotation, os)
	annotations := map[string]string{
		key: template.GetAnnotations()[key],
	}

	var workload *string
	if _, ok := template.Labels[fmt.Sprintf(templates.TemplateWorkloadLabel, serverWorkload)]; ok {
		workload = &serverWorkload
	} else if _, ok := template.Labels[fmt.Sprintf(templates.TemplateWorkloadLabel, desktopWorkload)]; ok {
		workload = &desktopWorkload
	}

	flavor := defaultFlavor
	// get workload label from the template
	labels := templates.OSLabelBuilder(&os, workload, &flavor)

	return labels, annotations, nil
}
