package templates

import (
	"fmt"
	"strings"

	"github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

const (
	// TemplateNamespace stores the default namespace for kubevirt templates
	TemplateNamespace = "openshift"
	defaultLinux      = "rhel8"
	defaultWindows    = "windows"
	defaultFlavor     = "medium"
)

// TemplateFinder attempts to find a template based on given parameters
type TemplateFinder struct {
	templateProvider templates.TemplateProvider
	osMapProvider    templates.OSMapProvider
}

// NewTemplateFinder creates new TemplateFinder
func NewTemplateFinder(templateProvider templates.TemplateProvider, osMapProvider templates.OSMapProvider) *TemplateFinder {
	return &TemplateFinder{
		templateProvider: templateProvider,
		osMapProvider:    osMapProvider,
	}
}

// FindTemplate attempts to find best match for a template based on the source VM
func (f *TemplateFinder) FindTemplate(vm *ovirtsdk.Vm) (*templatev1.Template, error) {
	os, err := f.findOperatingSystem(vm)
	if err != nil {
		return nil, err
	}
	workload := getWorkload(vm)
	return f.getTemplate(os, workload)
}

func (f *TemplateFinder) findOperatingSystem(vm *ovirtsdk.Vm) (string, error) {
	guestOsToCommon, osInfoToCommon, err := f.osMapProvider.GetOSMaps()
	if err != nil {
		return "", err
	}
	// Attempt resolving OS based on VM Guest OS information
	if gos, found := vm.GuestOperatingSystem(); found {
		distribution, _ := gos.Distribution()
		version, _ := gos.Version()
		fullVersion, _ := version.FullVersion()
		os, found := guestOsToCommon[distribution]
		if found {
			return fmt.Sprintf("%s%s", os, fullVersion), nil
		}
	}
	// Attempt resolving OS by looking for a match based on OS mapping
	if os, found := vm.Os(); found {
		osType, _ := os.Type()
		mappedOS, found := osInfoToCommon[osType]
		if found {
			return mappedOS, nil
		}

		// limit number of possibilities
		osType = strings.ToLower(osType)
		if strings.Contains(osType, "linux") || strings.Contains(osType, "rhel") {
			return defaultLinux, nil
		} else if strings.Contains(osType, "win") {
			return defaultWindows, nil
		}
	}
	// return empty to fail label selector
	return "", fmt.Errorf("Failed to find operating system for the VM")
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
	// Take first which matches label selector
	return &templates.Items[0], nil
}
