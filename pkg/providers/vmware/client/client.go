package client

import (
	"context"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"net/url"
)

// RichOvirtClient is responsible for retrieving VM data from oVirt API
type richVmwareClient struct {
	client *govmomi.Client
}

// NewRichVMwareClient creates new, connected rich vmware client. After it is no longer needed, call Close().
func NewRichVMWareClient(apiUrl, username, password string, insecure bool) (*richVmwareClient, error) {
	u, err := url.Parse(apiUrl)
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		u.User = url.UserPassword(username, password)
	}
	govmomiClient, err := govmomi.NewClient(context.TODO(), u, insecure)
	if err != nil {
		return nil, err
	}
	vmwareClient := richVmwareClient{
		client: govmomiClient,
	}
	return &vmwareClient, nil
}

func (r richVmwareClient) GetVM(id *string, _ *string, _ *string, _ *string) (interface{}, error) {
	return r.getVM(*id)
}

func (r richVmwareClient) getVM(id string) (*object.VirtualMachine, error) {
	vmRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: id}
	vm := object.NewVirtualMachine(r.client.Client, vmRef)
	return vm, nil
}

func (r richVmwareClient) StopVM(id string) error {
	vm, err := r.getVM(id)
	if err != nil {
		return err
	}
	powerState, err := vm.PowerState(context.TODO())
	if err != nil {
		return err
	}
	if powerState != types.VirtualMachinePowerStatePoweredOff {
		task, err := vm.PowerOff(context.TODO())
		if err != nil {
			return err
		}
		return task.Wait(context.TODO())
	}
	return nil
}

func (r richVmwareClient) StartVM(id string) error {
	vm, err := r.getVM(id)
	if err != nil {
		return err
	}
	powerState, err := vm.PowerState(context.TODO())
	if err != nil {
		return err
	}
	if powerState != types.VirtualMachinePowerStatePoweredOn {
		task, err := vm.PowerOn(context.TODO())
		if err != nil {
			return err
		}
		return task.Wait(context.TODO())
	}
	return nil
}

func (r richVmwareClient) Close() error {
	// nothing to do
	return nil
}
