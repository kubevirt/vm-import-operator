package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	// Number of minutes to wait for VM to be stopped
	shutdownTimeout = 5 * time.Minute
	// Vm poll interval in seconds
	pollInterval = 5 * time.Second
	// timeout value in seconds for vmware api requests
	timeout = 30 * time.Second
)

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

func (r RichVmwareClient) getVMByMoRef(moRef string) *object.VirtualMachine {
	ref := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: moRef,
	}
	return object.NewVirtualMachine(r.client, ref)
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
	if vmRef == nil {
		return nil, fmt.Errorf("vm '%s' not found", id)
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

// CreateVMSnapshot creates a snapshot of the VM.
func (r RichVmwareClient) CreateVMSnapshot(moRef string, name string, desc string, memory bool, quiesce bool) (*types.ManagedObjectReference, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm := r.getVMByMoRef(moRef)
	task, err := vm.CreateSnapshot(ctx, name, desc, memory, quiesce)
	if err != nil {
		return nil, err
	}
	res, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}
	snapshotRef := res.Result.(types.ManagedObjectReference)
	return &snapshotRef, nil
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
func (r RichVmwareClient) StartVM(moRef string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm := r.getVMByMoRef(moRef)
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
func (r RichVmwareClient) StopVM(moRef string) error {
	err := r.shutdownGuest(moRef)
	if err != nil {
		return err
	}

	// wait for the VM to shut down
	c := make(chan bool, 1)
	go func() {
		for {
			time.Sleep(pollInterval)
			powerState, err := r.powerState(moRef)
			if err != nil {
				c <- false
			}
			if powerState == types.VirtualMachinePowerStatePoweredOff {
				c <- true
				break
			}
		}
	}()

	select {
	case success := <-c:
		if !success {
			return fmt.Errorf("failed to gracefully shutdown vm %s", moRef)
		}
		return nil
	case <-time.After(shutdownTimeout):
		return fmt.Errorf("timed out trying to gracefully shutdown vm %s", moRef)
	}
}

func (r RichVmwareClient) shutdownGuest(moRef string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm := r.getVMByMoRef(moRef)

	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return err
	}

	if powerState == types.VirtualMachinePowerStatePoweredOff {
		return nil
	}

	return vm.ShutdownGuest(ctx)
}

func (r RichVmwareClient) powerState(moRef string) (types.VirtualMachinePowerState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	vm := r.getVMByMoRef(moRef)

	return vm.PowerState(ctx)
}

// TestConnection checks the connectivity to the vCenter or ESXi host.
func (r RichVmwareClient) TestConnection() error {
	_, err := r.client.Get(r.client.URL().String())
	return err
}

// Close logs out and shuts down idle connections.
func (r RichVmwareClient) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer r.client.CloseIdleConnections()

	return r.sessionManager.Logout(ctx)
}
