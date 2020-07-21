package ovirtclient

import (
	"fmt"
	"runtime/debug"
	"time"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

const (
	// Number of minutes to wait for VM to be stopped
	vmStopTimeout = 5
	// Vm poll interval in seconds
	vmPollInterval = 5
)

// ConnectionSettings wrap information required to make oVirt API connection
type ConnectionSettings struct {
	URL      string
	Username string
	Password string
	CACert   []byte
}

// RichOvirtClient is responsible for retrieving VM data from oVirt API
type richOvirtClient struct {
	connection *ovirtsdk.Connection
}

// NewRichOvirtClient creates new, connected rich oVirt client. After it is no longer needed, call Close().
func NewRichOvirtClient(cs *ConnectionSettings) (*richOvirtClient, error) {
	con, err := connect(cs.URL, cs.Username, cs.Password, cs.CACert)
	if err != nil {
		return nil, err
	}
	ovirtClient := richOvirtClient{
		connection: con,
	}
	return &ovirtClient, nil
}

// Close releases the resources used by this client.
func (client *richOvirtClient) Close() error {
	return client.connection.Close()
}

// GetVM retrieves oVirt VM data for given id or name and cluster. VM will have certain links followed and updated.
func (client *richOvirtClient) GetVM(id *string, name *string, cluster *string, clusterID *string) (_ interface{}, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("ovirt client panicked GetVM: %v", err)
			debug.PrintStack()
		}
	}()
	vm, err := client.fetchVM(id, name, cluster, clusterID)
	if err != nil {
		return nil, err
	}
	err = client.populateNics(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateDiskAttachments(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateCdRoms(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateFloppies(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateHostDevices(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateReportedDevices(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateQuota(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateWatchdogs(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateGraphicsConsoles(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateInstanceType(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateTags(vm)
	if err != nil {
		return nil, err
	}
	err = client.populateCluster(vm)
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// StopVM stop the VM and wait for the vm to be stopped
func (client *richOvirtClient) StopVM(id string) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("ovirt client panicked in StopVM: %v", err)
			debug.PrintStack()
		}
	}()
	vmService := client.connection.SystemService().VmsService().VmService(id)

	// Stop the VM gracefully:
	_, err := vmService.Shutdown().Send()
	if err != nil {
		return err
	}

	var vm *ovirtsdk.Vm
	var vmAvailable bool
	// Wait for VM to be stopped
	c := make(chan bool, 1)
	go func() {
		for {
			time.Sleep(vmPollInterval * time.Second)
			vmResponse, _ := vmService.Get().Send()
			vm, vmAvailable = vmResponse.Vm()
			if !vmAvailable {
				c <- false
			}
			if status, _ := vm.Status(); status == ovirtsdk.VMSTATUS_DOWN {
				c <- true
				break
			}
		}
	}()
	select {
	case success := <-c:
		if !success {
			return fmt.Errorf("Failed to stop vm %s", id)
		}
		return nil
	case <-time.After(vmStopTimeout * time.Minute):
		status := ovirtsdk.VMSTATUS_UNKNOWN
		if vm != nil {
			if vmStatus, ok := vm.Status(); ok {
				status = vmStatus
			}
		}
		return fmt.Errorf("Failed to stop vm %s, current status is %s", id, status)
	}
}

// StartVM requests VM start and doesn't wait for it to be UP
func (client *richOvirtClient) StartVM(id string) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("ovirt client panicked in StartVM: %v", err)
			debug.PrintStack()
		}
	}()
	vmService := client.connection.SystemService().VmsService().VmService(id)

	vmResponse, _ := vmService.Get().Send()
	vm, vmAvailable := vmResponse.Vm()
	if !vmAvailable {
		return fmt.Errorf("Failed to start vm %s, vm is not available", id)
	}

	// Request the VM startup if VM is not UP
	if status, _ := vm.Status(); status == ovirtsdk.VMSTATUS_DOWN {
		_, err := vmService.Start().Send()
		if err != nil {
			return err
		}
	}
	return nil
}

// TestConnection checks the connectivity to oVirt provider
func (client *richOvirtClient) TestConnection() error {
	return client.connection.Test()
}

func (client *richOvirtClient) fetchVM(id *string, name *string, clusterName *string, clusterID *string) (*ovirtsdk.Vm, error) {
	// Id of the VM specified:
	if id != nil {
		// We need to pass "All-content" header, so we fetch also information about console, etc.
		response, err := client.connection.SystemService().VmsService().VmService(*id).Get().AllContent(true).Send()
		if err != nil {
			return nil, err
		}
		if vm, ok := response.Vm(); ok {
			return vm, nil
		}
		return nil, fmt.Errorf("Virtual machine %v not found", *id)
	}

	// Cluster and name specified:
	var (
		response *ovirtsdk.VmsServiceListResponse
		err      error
	)
	if name == nil {
		return nil, fmt.Errorf("both ID and name of the VM are missing")
	}
	if clusterName != nil {
		response, err = client.connection.SystemService().VmsService().List().Search(fmt.Sprintf("name=%v and cluster=%v", *name, *clusterName)).Send()
	} else {
		response, err = client.connection.SystemService().VmsService().List().Search(fmt.Sprintf("name=%v", *name)).Send()
	}
	if err != nil {
		return nil, err
	}

	vms, _ := response.Vms()
	vmsCount := len(vms.Slice())
	if vmsCount == 1 {
		return vms.Slice()[0], nil
	} else if vmsCount > 1 {
		// If user specified clusterID, iterate over list of VMs and find the VM
		// that match the clusterID
		if clusterID != nil {
			for _, vm := range vms.Slice() {
				if vmID, _ := vm.Id(); vmID == *clusterID {
					return vm, nil
				}
			}
		}
		return nil, fmt.Errorf("Found more than one virtual machine with name %v in cluster %v(%v). VM IDs: %v", *name, clusterName, clusterID, getVMIDs(vms.Slice()))
	}
	return nil, fmt.Errorf("Virtual machine %v not found in cluster: %v(%v)", *name, clusterName, clusterID)
}

func (client *richOvirtClient) populateHostDevices(vm *ovirtsdk.Vm) error {
	if hostDevices, ok := vm.HostDevices(); ok {
		followed, err := client.connection.FollowLink(hostDevices)
		if err != nil {
			return err
		}
		vm.SetHostDevices(followed.(*ovirtsdk.HostDeviceSlice))
	}
	return nil
}

func (client *richOvirtClient) populateReportedDevices(vm *ovirtsdk.Vm) error {
	if reportedDevices, ok := vm.ReportedDevices(); ok {
		followed, err := client.connection.FollowLink(reportedDevices)
		if err != nil {
			return err
		}
		vm.SetReportedDevices(followed.(*ovirtsdk.ReportedDeviceSlice))
	}
	return nil
}

func (client *richOvirtClient) populateCdRoms(vm *ovirtsdk.Vm) error {
	if cdroms, ok := vm.Cdroms(); ok {
		followed, err := client.connection.FollowLink(cdroms)
		if err != nil {
			return err
		}
		vm.SetCdroms(followed.(*ovirtsdk.CdromSlice))
	}
	return nil
}

func (client *richOvirtClient) populateFloppies(vm *ovirtsdk.Vm) error {
	if floppies, ok := vm.Floppies(); ok {
		followed, err := client.connection.FollowLink(floppies)
		if err != nil {
			return err
		}
		vm.SetFloppies(followed.(*ovirtsdk.FloppySlice))
	}
	return nil
}

func (client *richOvirtClient) populateWatchdogs(vm *ovirtsdk.Vm) error {
	if watchDogs, ok := vm.Watchdogs(); ok {
		followed, err := client.connection.FollowLink(watchDogs)
		if err != nil {
			return err
		}
		vm.SetWatchdogs(followed.(*ovirtsdk.WatchdogSlice))
	}
	return nil
}

func (client *richOvirtClient) populateGraphicsConsoles(vm *ovirtsdk.Vm) error {
	if consoles, ok := vm.GraphicsConsoles(); ok {
		followed, err := client.connection.FollowLink(consoles)
		if err != nil {
			return err
		}
		vm.SetGraphicsConsoles(followed.(*ovirtsdk.GraphicsConsoleSlice))
	}
	return nil
}

func (client *richOvirtClient) populateQuota(vm *ovirtsdk.Vm) error {
	if quota, ok := vm.Quota(); ok {
		// Quota might not have Href populater. See: https://bugzilla.redhat.com/show_bug.cgi?id=1814613
		if _, ok := quota.Href(); ok {
			followed, err := client.connection.FollowLink(quota)
			if err != nil {
				return err
			}
			vm.SetQuota(followed.(*ovirtsdk.Quota))
		}
	}
	return nil
}

func (client *richOvirtClient) populateNics(vm *ovirtsdk.Vm) error {
	if nics, ok := vm.Nics(); ok {
		followed, err := client.connection.FollowLink(nics)
		nics = followed.(*ovirtsdk.NicSlice)
		if err != nil {
			return err
		}
		for _, nic := range nics.Slice() {
			if prof, ok := nic.VnicProfile(); ok {
				vnicFollowed, err := client.connection.FollowLink(prof)
				if err != nil {
					return err
				}
				vnic := vnicFollowed.(*ovirtsdk.VnicProfile)
				if net, ok := vnic.Network(); ok {
					network, err := client.connection.FollowLink(net)
					if err != nil {
						return err
					}
					vnic.SetNetwork(network.(*ovirtsdk.Network))
				}
				nic.SetVnicProfile(vnic)
			}
			if net, ok := nic.Network(); ok {
				network, err := client.connection.FollowLink(net)
				if err != nil {
					return err
				}
				nic.SetNetwork(network.(*ovirtsdk.Network))
			}
		}
		vm.SetNics(nics)
	}
	return nil
}

func (client *richOvirtClient) populateDiskAttachments(vm *ovirtsdk.Vm) error {
	if das, ok := vm.DiskAttachments(); ok {
		followed, err := client.connection.FollowLink(das)
		if err != nil {
			return err
		}
		attachments := followed.(*ovirtsdk.DiskAttachmentSlice)
		for _, da := range attachments.Slice() {
			if disk, ok := da.Disk(); ok {
				// Follow disk:
				diskFollowed, err := client.connection.FollowLink(disk)
				if err != nil {
					return err
				}
				diskPopulated := diskFollowed.(*ovirtsdk.Disk)

				// Follow storage domain:
				if storageDomains, ok := diskPopulated.StorageDomains(); ok {
					storageDomainHref := storageDomains.Slice()[0]
					storageDomainPopulated, err := client.connection.FollowLink(storageDomainHref)
					if err != nil {
						return err
					}
					diskPopulated.SetStorageDomain(storageDomainPopulated.(*ovirtsdk.StorageDomain))
				}
				da.SetDisk(diskPopulated)
			}
		}
		vm.SetDiskAttachments(attachments)
	}
	return nil
}

func (client *richOvirtClient) populateInstanceType(vm *ovirtsdk.Vm) error {
	if instanceType, ok := vm.InstanceType(); ok {
		followed, err := client.connection.FollowLink(instanceType)
		if err != nil {
			return err
		}
		vm.SetInstanceType(followed.(*ovirtsdk.InstanceType))
	}
	return nil
}

func (client *richOvirtClient) populateTags(vm *ovirtsdk.Vm) error {
	if tags, ok := vm.Tags(); ok {
		followed, err := client.connection.FollowLink(tags)
		if err != nil {
			return err
		}
		tagsPopulated := followed.(*ovirtsdk.TagSlice)
		vm.SetTags(tagsPopulated)
	}
	return nil
}

func (client *richOvirtClient) populateCluster(vm *ovirtsdk.Vm) error {
	if cluster, ok := vm.Cluster(); ok {
		followed, err := client.connection.FollowLink(cluster)
		if err != nil {
			return err
		}
		vm.SetCluster(followed.(*ovirtsdk.Cluster))
	}
	return nil
}

func connect(apiURL string, username string, password string, caCrt []byte) (*ovirtsdk.Connection, error) {
	connection, err := ovirtsdk.NewConnectionBuilder().
		URL(apiURL).
		Username(username).
		Password(password).
		CACert(caCrt).
		Build()
	return connection, err
}

func getVMIDs(vms []*ovirtsdk.Vm) []string {
	ids := make([]string, len(vms))
	for i := range vms {
		ids[i] = vms[i].MustId()
	}
	return ids
}
