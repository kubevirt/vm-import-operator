package templates

import (
	"fmt"
	"sort"

	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/vmware/govmomi/vim25/mo"
)

var (
	templateNamespace = "openshift"
	serverWorkload    = "server"
	desktopWorkload   = "desktop"
	smallFlavor       = "small"
	mediumFlavor      = "medium"
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

	// look for a small template first, then look for a medium template
	// if neither a small server nor desktop template can be found
	var template *templatev1.Template

loop:
	for _, flavor := range []string{smallFlavor, mediumFlavor} {
		for _, workload := range []string{serverWorkload, desktopWorkload} {
			tmpls, err := f.templateProvider.Find(&templateNamespace, &os, &workload, &flavor)
			if err != nil {
				return nil, err
			}

			if len(tmpls.Items) == 0 {
				continue
			} else {
				// Take first which matches label selector
				sort.Slice(tmpls.Items, func(i, j int) bool {
					return tmpls.Items[j].CreationTimestamp.Before(&tmpls.Items[i].CreationTimestamp)
				})
				template = &tmpls.Items[0]
				break loop
			}
		}
	}

	if template == nil {
		return nil, fmt.Errorf("template not found for %s OS", os)
	}

	return template, nil
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

	// get workload label from the template
	var workload *string
	if _, ok := template.Labels[fmt.Sprintf(templates.TemplateWorkloadLabel, serverWorkload)]; ok {
		workload = &serverWorkload
	} else if _, ok := template.Labels[fmt.Sprintf(templates.TemplateWorkloadLabel, desktopWorkload)]; ok {
		workload = &desktopWorkload
	}

	// get flavor label from the template
	var flavor *string
	if _, ok := template.Labels[fmt.Sprintf(templates.TemplateFlavorLabel, smallFlavor)]; ok {
		flavor = &smallFlavor
	} else if _, ok := template.Labels[fmt.Sprintf(templates.TemplateFlavorLabel, mediumFlavor)]; ok {
		flavor = &mediumFlavor
	}

	labels := templates.OSLabelBuilder(&os, workload, flavor)

	return labels, annotations, nil
}
