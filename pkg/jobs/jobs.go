package jobs

import (
	"context"
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	batchv1 "k8s.io/api/batch/v1"

)

const (
	prefix       = "vmimport.v2v.kubevirt.io"
	vmiNameLabel = prefix + "/vmi-name"
)

// Manager provides operations on batch Jobs
type Manager struct {
	client client.Client
}

// NewManager creates new Job manager
func NewManager(client client.Client) Manager {
	return Manager{client: client}
}

// FindFor retrieves a Job matching given labels. If none can be found, both error and pointer will be nil. When there is more than 1 matching Job, error will be returned.
func (m *Manager) FindFor(vmiCrName types.NamespacedName) (*batchv1.Job, error) {
	jobList := batchv1.JobList{}
	labels := client.MatchingLabels{
		vmiNameLabel: utils.EnsureLabelValueLength(vmiCrName.Name),
	}

	err := m.client.List(context.TODO(), &jobList, labels, client.InNamespace(vmiCrName.Namespace))
	if err != nil {
		return nil, err
	}
	switch items := jobList.Items; len(items) {
	case 1:
		return &items[0], nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("too many jobs matching given labels: %v", labels)
	}
}

// CreateFor creates given Job, overriding given Name with a generated one. The Job will be associated with vmiCrName.
func (m *Manager) CreateFor(job *batchv1.Job, vmiCrName types.NamespacedName) error {
	job.Namespace = vmiCrName.Namespace
	// Force generation
	job.GenerateName = prefix
	job.Name = ""

	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}
	job.Labels[vmiNameLabel] = utils.EnsureLabelValueLength(vmiCrName.Name)

	return m.client.Create(context.TODO(), job)
}

// DeleteFor removes the Job created for vmiCrName
func (m *Manager) DeleteFor(vmiCrName types.NamespacedName) error {
	job, err := m.FindFor(vmiCrName)
	if err != nil {
		return err
	}
	if job != nil {
		return m.client.Delete(context.TODO(), job)
	}
	return nil
}

