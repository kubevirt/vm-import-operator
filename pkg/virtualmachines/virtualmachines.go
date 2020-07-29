package virtualmachines

import (
	"context"
	"fmt"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prefix       = "vmimport.v2v.kubevirt.io"
	vmiNameLabel = prefix + "/vmi-name"
)

// Manager provides operations on virtualmachines
type Manager struct {
	client client.Client
}

// NewManager creates new virtualmachines manager
func NewManager(client client.Client) Manager {
	return Manager{client: client}
}

// FindFor return virtualmachine for the VM import object
func (m *Manager) FindFor(vmiCrName types.NamespacedName) (*kubevirtv1.VirtualMachine, error) {
	instance := &v2vv1.VirtualMachineImport{}
	if err := m.client.Get(context.TODO(), vmiCrName, instance); err != nil {
		return nil, err
	}

	if instance.Status.TargetVMName == "" {
		return nil, fmt.Errorf("Virtual machine can't be found because it wasn't created, yet")
	}

	vm := &kubevirtv1.VirtualMachine{}
	if err := m.client.Get(context.TODO(), types.NamespacedName{Name: instance.Status.TargetVMName, Namespace: instance.Namespace}, vm); err != nil {
		return nil, err
	}

	return vm, nil
}

// DeleteFor removes virtualmachine associated with vmiCrName.
func (m *Manager) DeleteFor(vmiCrName types.NamespacedName) error {
	vm, err := m.FindFor(vmiCrName)
	if err != nil {
		return err
	}
	if err := m.client.Delete(context.TODO(), vm); err != nil {
		return err
	}
	return nil
}
