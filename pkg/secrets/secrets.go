package secrets

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

// Manager provides operations on secrets
type Manager struct {
	client client.Client
}

// NewManager creates new secrets manager
func NewManager(client client.Client) Manager {
	return Manager{client: client}
}

// FindFor retrieves secret associated with vmiCrName. If none can be found, both error and pointer will be nil. When there is more than 1 matching secret, error will be returned.
func (m *Manager) FindFor(vmiCrName types.NamespacedName) (*corev1.Secret, error) {
	secretList := corev1.SecretList{}
	labels := client.MatchingLabels{
		vmiNameLabel: utils.EnsureLabelValueLength(vmiCrName.Name),
	}

	err := m.client.List(context.TODO(), &secretList, labels, client.InNamespace(vmiCrName.Namespace))
	if err != nil {
		return nil, err
	}
	switch items := secretList.Items; len(items) {
	case 1:
		return &items[0], nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("too many secrets matching given labels: %v", labels)
	}
}

// CreateFor creates given secret, overriding given Name with a generated one. The secret will be associated with vmiCrName.
func (m *Manager) CreateFor(secret *corev1.Secret, vmiCrName types.NamespacedName) error {
	secret.Namespace = vmiCrName.Namespace
	// Force generation
	secret.GenerateName = prefix
	secret.Name = ""

	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[vmiNameLabel] = utils.EnsureLabelValueLength(vmiCrName.Name)

	return m.client.Create(context.TODO(), secret)
}

// DeleteFor removes secret associated with vmiCrName.
func (m *Manager) DeleteFor(vmiCrName types.NamespacedName) error {
	secret, err := m.FindFor(vmiCrName)
	if err != nil {
		return err
	}
	if secret != nil {
		return m.client.Delete(context.TODO(), secret)
	}
	return nil
}
