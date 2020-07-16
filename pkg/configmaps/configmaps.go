package configmaps

import (
	"context"
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/utils"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prefix       = "vmimport.v2v.kubevirt.io"
	vmiNameLabel = prefix + "/vmi-name"
)

// Manager provides operations on config maps
type Manager struct {
	client client.Client
}

// NewManager creates new config map manager
func NewManager(client client.Client) Manager {
	return Manager{client: client}
}

// FindFor retrieves config map matching given labels. If none can be found, both error and pointer will be nil. When there is more than 1 matching config map, error will be returned.
func (m *Manager) FindFor(vmiCrName types.NamespacedName) (*corev1.ConfigMap, error) {
	mapList := corev1.ConfigMapList{}
	labels := client.MatchingLabels{
		vmiNameLabel: utils.EnsureLabelValueLength(vmiCrName.Name),
	}

	err := m.client.List(context.TODO(), &mapList, labels, client.InNamespace(vmiCrName.Namespace))
	if err != nil {
		return nil, err
	}
	switch items := mapList.Items; len(items) {
	case 1:
		return &items[0], nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("too many config maps matching given labels: %v", labels)
	}
}

// CreateFor creates given config map, overriding given Name with a generated one. The config map will be associated with vmiCrName.
func (m *Manager) CreateFor(configMap *corev1.ConfigMap, vmiCrName types.NamespacedName) error {
	configMap.Namespace = vmiCrName.Namespace
	// Force generation
	configMap.GenerateName = prefix
	configMap.Name = ""

	if configMap.Labels == nil {
		configMap.Labels = make(map[string]string)
	}
	configMap.Labels[vmiNameLabel] = utils.EnsureLabelValueLength(vmiCrName.Name)

	return m.client.Create(context.TODO(), configMap)
}

// DeleteFor removes config map created for vmiCrName
func (m *Manager) DeleteFor(vmiCrName types.NamespacedName) error {
	configMap, err := m.FindFor(vmiCrName)
	if err != nil {
		return err
	}
	if configMap != nil {
		return m.client.Delete(context.TODO(), configMap)
	}
	return nil
}
