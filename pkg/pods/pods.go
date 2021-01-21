package pods

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/kubevirt/vm-import-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prefix       = "vmimport.v2v.kubevirt.io"
	vmiNameLabel = prefix + "/vmi-name"
)

// Manager provides operations on Pods
type Manager struct {
	client client.Client
}

// NewManager creates new Pod manager
func NewManager(client client.Client) Manager {
	return Manager{client: client}
}

// FindFor retrieves a Pod matching given labels. If none can be found, both error and pointer will be nil. When there is more than 1 matching Pod, error will be returned.
func (m *Manager) FindFor(vmiCrName types.NamespacedName) (*corev1.Pod, error) {
	podList := corev1.PodList{}
	labels := client.MatchingLabels{
		vmiNameLabel: utils.EnsureLabelValueLength(vmiCrName.Name),
	}

	err := m.client.List(context.TODO(), &podList, labels, client.InNamespace(vmiCrName.Namespace))
	if err != nil {
		return nil, err
	}
	switch items := podList.Items; len(items) {
	case 1:
		return &items[0], nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("too many pods matching given labels: %v", labels)
	}
}

// CreateFor creates given Pod, overriding given Name with a generated one. The Pod will be associated with vmiCrName.
func (m *Manager) CreateFor(pod *corev1.Pod, vmiCrName types.NamespacedName) error {
	pod.Namespace = vmiCrName.Namespace
	// Force generation
	pod.GenerateName = prefix
	pod.Name = ""

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[vmiNameLabel] = utils.EnsureLabelValueLength(vmiCrName.Name)

	return m.client.Create(context.TODO(), pod)
}

// DeleteFor removes the Pod created for vmiCrName
func (m *Manager) DeleteFor(vmiCrName types.NamespacedName) error {
	pod, err := m.FindFor(vmiCrName)
	if err != nil {
		return err
	}
	if pod != nil {
		foreground := metav1.DeletePropagationForeground
		opts := &client.DeleteOptions{PropagationPolicy: &foreground}
		return m.client.Delete(context.TODO(), pod, opts)
	}
	return nil
}
