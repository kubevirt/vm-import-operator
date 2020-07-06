package client

import (
	"context"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"net/url"
	"time"
)

// timeout value in seconds for vmware api requests
const timeout = 5 * time.Second

// RichOvirtClient is responsible for retrieving VM data from oVirt API
type RichVmwareClient struct {
	client *govmomi.Client
}

// NewRichVMwareClient creates new, connected rich vmware client. After it is no longer needed, call Close().
func NewRichVMWareClient(apiUrl, username, password string, insecure bool) (*RichVmwareClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	u, err := url.Parse(apiUrl)
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		u.User = url.UserPassword(username, password)
	}

	govmomiClient, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, err
	}
	vmwareClient := RichVmwareClient{
		client: govmomiClient,
	}
	return &vmwareClient, nil
}

func (r RichVmwareClient) GetVM(id *string, _ *string, _ *string, _ *string) (interface{}, error) {
	return r.getVM(*id), nil
}

func (r RichVmwareClient) getVM(id string) *object.VirtualMachine {
	vmRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: id}
	vm := object.NewVirtualMachine(r.client.Client, vmRef)
	return vm
}

func (r RichVmwareClient) GetVMProperties(vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	vmProperties := &mo.VirtualMachine{}
	err := vm.Properties(ctx, vm.Reference(), nil, vmProperties)
	if err != nil {
		return nil, err
	}
	return vmProperties, nil
}

func (r RichVmwareClient) GetVMHostProperties(vm *object.VirtualMachine) (*mo.HostSystem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	hostSystem, err := vm.HostSystem(ctx)
	if err != nil {
		return nil, err
	}

	hostProperties := &mo.HostSystem{}
	err = hostSystem.Properties(context.TODO(), hostSystem.Reference(), nil, hostProperties)
	if err != nil {
		return nil, err
	}

	return hostProperties, nil
}

func (r RichVmwareClient) StartVM(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm := r.getVM(id)
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return err
	}
	if powerState != types.VirtualMachinePowerStatePoweredOn {
		task, err := vm.PowerOn(ctx)
		if err != nil {
			return err
		}
		return task.Wait(ctx)
	}
	return nil
}

func (r RichVmwareClient) StopVM(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm := r.getVM(id)
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return err
	}
	if powerState != types.VirtualMachinePowerStatePoweredOff {
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return err
		}
		return task.Wait(ctx)
	}
	return nil
}

func (r RichVmwareClient) Close() error {
	// nothing to do
	return nil
}
