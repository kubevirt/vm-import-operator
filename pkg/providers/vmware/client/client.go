package client

import (
	"context"
	"errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"net/url"
	"time"
)

// timeout value in seconds for vmware api requests
const timeout = 30 * time.Second

// RichVmwareClient is responsible for retrieving VM data from the VMware API.
type RichVmwareClient struct {
	client         *vim25.Client
	user           *url.Userinfo
	sessionManager *session.Manager
}

// NewRichVMWareClient creates a new, connected rich VMWare client.
func NewRichVMWareClient(apiUrl, username, password string, thumbprint string) (*RichVmwareClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	u, err := url.Parse(apiUrl)
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		u.User = url.UserPassword(username, password)
	}

	soapClient := soap.NewClient(u, false)
	soapClient.SetThumbprint(u.Host, thumbprint)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	sessionManager := session.NewManager(vimClient)
	err = sessionManager.Login(ctx, u.User)
	if err != nil {
		return nil, err
	}

	vmwareClient := RichVmwareClient{
		client:         vimClient,
		user:           u.User,
		sessionManager: sessionManager,
	}
	return &vmwareClient, nil
}

// GetVM retrieves a VM from a vCenter or ESXI host by UUID or by name/inventory path.
func (r RichVmwareClient) GetVM(id *string, name *string, _ *string, _ *string) (interface{}, error) {
	if id != nil {
		return r.getVMByUUID(*id)
	}
	if name != nil {
		return r.getVMByInventoryPath(*name)
	}
	return nil, errors.New("not found")
}

// getVMByUUID gets a VM by its UUID
func (r RichVmwareClient) getVMByUUID(id string) (*object.VirtualMachine, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	searchIndex := object.NewSearchIndex(r.client)
	instanceUUID := false
	vmRef, err := searchIndex.FindByUuid(ctx, nil, id, true, &instanceUUID)
	if err != nil {
		return nil, err
	}
	vm := object.NewVirtualMachine(r.client, vmRef.Reference())
	return vm, nil
}

// getVMByInventoryPath gets a VM by its complete inventory path or by name alone.
func (r RichVmwareClient) getVMByInventoryPath(vmPath string) (*object.VirtualMachine, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	finder := find.NewFinder(r.client)

	vm, err := finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// GetVMProperties retrieves the Properties struct for the VM.
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

// GetVMHostProperties retrieves the Properties struct for the HostSystem the VM is on.
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

// StartVM requests VM start and doesn't wait for it to complete.
func (r RichVmwareClient) StartVM(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm, err := r.getVMByUUID(id)
	if err != nil {
		return err
	}
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return err
	}
	if powerState != types.VirtualMachinePowerStatePoweredOn {
		_, err := vm.PowerOn(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// StopVM stops the VM and waits for the vm to be stopped.
func (r RichVmwareClient) StopVM(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm, err := r.getVMByUUID(id)
	if err != nil {
		return err
	}
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

// TestConnection checks the connectivity to the vCenter or ESXi host.
func (r RichVmwareClient) TestConnection() error {
	_, err := r.client.Get(r.client.URL().String())
	return err
}

// Close is a no-op which is present in order to satisfy the VMClient interface.
func (r RichVmwareClient) Close() error {
	// nothing to do
	return nil
}
