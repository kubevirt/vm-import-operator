package templates

import (
	"fmt"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	tempclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// UsedTemplateLabel is a label to be added to the VM specifying which template used to created it
	UsedTemplateLabel = "vm.kubevirt.io/template"

	// TemplateOsLabel is a label that specifies the OS of the template
	TemplateOsLabel = "os.template.kubevirt.io/%s"

	// TemplateWorkloadLabel is a label that specifies the workload of the template
	TemplateWorkloadLabel = "workload.template.kubevirt.io/%s"

	// TemplateFlavorLabel is a label that specifies the flavor of the template
	TemplateFlavorLabel = "flavor.template.kubevirt.io/%s"
)

// Templates is responsible for finding templates
type Templates struct {
	Client *tempclient.TemplateV1Client
}

// Find looks for a template based on given namespace and options
func (t *Templates) Find(
	namespace *string,
	os *string,
	workload *string,
	flavor *string,
) (*templatev1.TemplateList, error) {
	labelSelector := OSLabelSelectorBuilder(os, workload, nil)
	options := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	return t.Client.Templates(*namespace).List(options)
}

// OSLabelSelectorBuilder build the label selector based on template criteria
func OSLabelSelectorBuilder(os *string, workload *string, flavor *string) string {
	labeles := OSLabelBuilder(os, workload, flavor)
	keys := make([]string, 0, len(labeles))
	for k := range labeles {
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}

// OSLabelBuilder builds template labels based on template criteria
func OSLabelBuilder(os *string, workload *string, flavor *string) map[string]string {
	labels := make(map[string]string)
	if os != nil {
		labels[fmt.Sprintf(TemplateOsLabel, *os)] = "true"
	}
	if workload != nil {
		labels[fmt.Sprintf(TemplateWorkloadLabel, *workload)] = "true"
	}
	if flavor != nil {
		labels[fmt.Sprintf(TemplateFlavorLabel, *flavor)] = "true"
	}
	return labels
}
