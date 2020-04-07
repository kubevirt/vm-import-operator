package templates

import (
	"fmt"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
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
	templateProvider TemplateProvider
	osMapProvider    OSMapProvider
}

// TemplateProvider searches for template in Openshift
type TemplateProvider interface {
	Find(namespace *string, os *string, workload *string, flavor *string) (*templatev1.TemplateList, error)
	Process(namespace string, vmName string, template *templatev1.Template) (*templatev1.Template, error)
}

// NewTemplateFinder creates new TemplateFinder
func NewTemplateFinder(templateProvider TemplateProvider, osMapProvider OSMapProvider) *TemplateFinder {
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

// ProcessTemplate processes template with provided parameter values
func (f *TemplateFinder) ProcessTemplate(template *templatev1.Template, vmName string) (*kubevirtv1.VirtualMachine, error) {
	processed, err := f.templateProvider.Process(TemplateNamespace, vmName, template)
	if err != nil {
		return nil, err
	}
	var vm = &kubevirtv1.VirtualMachine{}
	for _, obj := range processed.Objects {
		decoder := kubevirtv1.Codecs.UniversalDecoder(kubevirtv1.GroupVersion)
		decoded, err := runtime.Decode(decoder, obj.Raw)
		if err != nil {
			return nil, err
		}
		done, ok := decoded.(*kubevirtv1.VirtualMachine)
		if ok {
			vm = done
			break
		}
	}
	if len(vm.Spec.Template.Spec.Volumes) > 0 {
		vm.Spec.Template.Spec.Volumes = []kubevirtv1.Volume{}
	}
	if len(vm.Spec.Template.Spec.Networks) > 0 {
		vm.Spec.Template.Spec.Networks = []kubevirtv1.Network{}
	}
	if len(vm.Spec.DataVolumeTemplates) > 0 {
		vm.Spec.DataVolumeTemplates = []cdiv1.DataVolume{}
	}
	return vm, nil
}
