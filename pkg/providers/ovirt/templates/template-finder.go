package templates

import (
	"fmt"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	tempclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	templateNamespace = "openshift"
	defaultLinux      = "rhel8"
	defaultWindows    = "windows"
)

var osInfo = map[string]string{
	"Red Hat Enterprise Linux Server": "rhel",
	// TODO add more
}

// TemplateFinder attempts to find a template based on given parameters
type TemplateFinder struct {
	client *tempclient.TemplateV1Client
}

// NewTemplateFinder creates new TemplateFinder
func NewTemplateFinder(tempClient *tempclient.TemplateV1Client) *TemplateFinder {
	return &TemplateFinder{
		client: tempClient,
	}
}

// FindTemplate attempts to find best match for a template based on the source VM
func (f *TemplateFinder) FindTemplate(vm *ovirtsdk.Vm) (*templatev1.Template, error) {
	os := findOperatingSystem(vm)
	workload := getWorkload(vm)
	return f.getTemplate(os, workload)
}

func findOperatingSystem(vm *ovirtsdk.Vm) string {
	if gos, found := vm.GuestOperatingSystem(); found {
		distribution, _ := gos.Distribution()
		version, _ := gos.Version()
		fullVersion, _ := version.FullVersion()
		return fmt.Sprintf("%s%s", osInfo[distribution], fullVersion)
	}
	if os, found := vm.Os(); found {
		osType, _ := os.Type()
		// limit number of possibilities
		osType = strings.ToLower(osType)
		if strings.Contains(osType, "linux") || strings.Contains(osType, "rhel") {
			return defaultLinux
		} else if strings.Contains(osType, "win") {
			return defaultWindows
		}
	}
	// return empty to fail label selector
	return ""
}

func getWorkload(vm *ovirtsdk.Vm) string {
	// vm type is always available
	vmType, _ := vm.Type()
	// we need to remove underscore from high_performance, other workloads are OK
	return strings.Replace(string(vmType), "_", "", -1)
}

func (f *TemplateFinder) getTemplate(os string, workload string) (*templatev1.Template, error) {
	// We update metadata from the source vm so we default to medium flavor
	labelSelector := fmt.Sprintf("os.template.kubevirt.io/%s=true,workload.template.kubevirt.io/%s=true,flavor.template.kubevirt.io/medium=true", os, workload)
	options := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	templates, err := f.client.Templates(templateNamespace).List(options)
	if err != nil {
		return nil, err
	}
	if len(templates.Items) == 0 {
		return nil, fmt.Errorf("Template not found for %s OS and %s workload", os, workload)
	}
	// Take first which matches label selector
	return &templates.Items[0], nil
}
