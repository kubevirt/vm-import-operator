package templates

import (
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	// TemplateNamespace stores the default namespace for kubevirt templates
	TemplateNamespace = "openshift"

	// templateNameLabel defines a label of the template name which was used to created the VM
	templateNameLabel = "vm.kubevirt.io/template"
)

// TemplateHandler attempts to process templates based on given parameters
type TemplateHandler struct {
	templateProvider TemplateProvider
}

// NewTemplateHandler creates new TemplateHandler
func NewTemplateHandler(templateProvider TemplateProvider) *TemplateHandler {
	return &TemplateHandler{
		templateProvider: templateProvider,
	}
}

// ProcessTemplate processes template with provided parameter values
func (f *TemplateHandler) ProcessTemplate(template *templatev1.Template, vmName string) (*kubevirtv1.VirtualMachine, error) {
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
	addLabels(vm, template)
	return vm, nil
}

func addLabels(vm *kubevirtv1.VirtualMachine, template *templatev1.Template) {
	labels := vm.ObjectMeta.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
		vm.ObjectMeta.SetLabels(labels)
	}
	labels[templateNameLabel] = template.GetObjectMeta().GetName()
}
