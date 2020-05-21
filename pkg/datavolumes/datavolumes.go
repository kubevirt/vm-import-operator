package datavolumes

import (
	"context"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manager provides operations on datavolumes
type Manager struct {
	client client.Client
}

// NewManager creates new datavolumes manager
func NewManager(client client.Client) Manager {
	return Manager{client: client}
}

// FindFor return list od datavolumes for the VM import object
func (m *Manager) FindFor(vmiCrName types.NamespacedName) ([]*cdiv1.DataVolume, error) {
	instance := &v2vv1alpha1.VirtualMachineImport{}
	if err := m.client.Get(context.TODO(), vmiCrName, instance); err != nil {
		return nil, err
	}

	var errs []error
	var dvs []*cdiv1.DataVolume
	for _, dvID := range instance.Status.DataVolumes {
		dv := &cdiv1.DataVolume{}
		if err := m.client.Get(context.TODO(), types.NamespacedName{Name: dvID.Name, Namespace: instance.Namespace}, dv); err != nil {
			errs = append(errs, err)
		}
		dvs = append(dvs, dv)
	}

	if len(errs) > 0 {
		return dvs, errors.Errorf("Find of datavolumes of VM import %s/%s failed: %s", vmiCrName.Name, vmiCrName.Namespace, errs)
	}

	return dvs, nil
}

// DeleteFor removes datavolumes associated with vmiCrName.
func (m *Manager) DeleteFor(vmiCrName types.NamespacedName) error {
	var errs []error
	dvs, err := m.FindFor(vmiCrName)
	if dvs != nil {
		for _, dv := range dvs {
			if err := m.client.Delete(context.TODO(), dv); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errors.Errorf("Delete of datavolumes of VM import %s/%s failed: %s", vmiCrName.Name, vmiCrName.Namespace, err)
	}

	return nil
}
