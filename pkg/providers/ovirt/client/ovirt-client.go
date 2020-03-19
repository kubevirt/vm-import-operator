package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

// OvirtClient retrieves rich VM object - with all necessary links have been followed
type OvirtClient interface {
	GetVM(id *string, name *string, cluster *string) (*ovirtsdk.Vm, error)
	Close() error
}

// ConnectionSettings wrap information required to make oVirt API connection
type ConnectionSettings struct {
	URL      string
	Username string
	Password string
	Insecure bool
	CAFile   string
}

// RichOvirtClient is responsilbe for retrieving VM data from oVirt API
type richOvirtClient struct {
	connection *ovirtsdk.Connection
}

// NewRitchOvirtClient creates new, connected rich oVirt client. After it is no longer needed, call Close().
func NewRitchOvirtClient(cs *ConnectionSettings) (OvirtClient, error) {
	con, err := connect(cs.URL, cs.Username, cs.Password, cs.CAFile, cs.Insecure)
	if err != nil {
		return nil, err
	}
	client := richOvirtClient{
		connection: con,
	}
	return &client, nil
}

// Close releases the resources used by this client.
func (client *richOvirtClient) Close() error {
	return client.connection.Close()
}

// GetVM rertrieves oVirt VM data for given id or name and cluster. VM will have certain links followed and updated.
func (client *richOvirtClient) GetVM(id *string, name *string, cluster *string) (*ovirtsdk.Vm, error) {
	vm, err := client.fetchVM(id, name, cluster)
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
	return vm, nil
}

func (client *richOvirtClient) fetchVM(id *string, name *string, cluster *string) (*ovirtsdk.Vm, error) {
	// Id of the VM specified:
	if id != nil {
		response, err := client.connection.SystemService().VmsService().VmService(*id).Get().Send()
		if err != nil {
			return nil, err
		}
		if vm, ok := response.Vm(); ok {
			return vm, nil
		}
		return nil, fmt.Errorf("Virtual machine %v not found", *id)
	}
	// Cluster/name of the VM specified:
	response, err := client.connection.SystemService().VmsService().List().Search(fmt.Sprintf("name=%v and cluster=%v", *name, *cluster)).Send()
	if err != nil {
		return nil, err
	}
	vms, _ := response.Vms()
	vmsCount := len(vms.Slice())
	if vmsCount == 1 {
		return vms.Slice()[0], nil
	} else if vmsCount > 1 {
		return nil, fmt.Errorf("Found more than one virtual machine with name %v in clusters with name %v. VM IDs: %v", *name, *cluster, getVMIDs(vms.Slice()))
	}
	return nil, fmt.Errorf("Virtual machine %v not found in cluster %v", *name, *cluster)
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
				diskFollowed, err := client.connection.FollowLink(disk)
				if err != nil {
					return err
				}
				da.SetDisk(diskFollowed.(*ovirtsdk.Disk))
			}
		}
		vm.SetDiskAttachments(attachments)
	}
	return nil
}

func connect(apiURL string, username string, password string, caFile string, insecure bool) (*ovirtsdk.Connection, error) {
	connection, err := ovirtsdk.NewConnectionBuilder().
		URL(apiURL).
		Username(username).
		Password(password).
		CAFile(caFile).
		Insecure(insecure).
		Build()
	return connection, err
}

func getVMIDs(vms []*ovirtsdk.Vm) []string {
	ids := make([]string, len(vms), len(vms))
	for i := range vms {
		ids[i] = vms[i].MustId()
	}
	return ids
}
