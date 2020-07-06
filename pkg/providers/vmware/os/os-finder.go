package os

import (
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/os"
	"github.com/vmware/govmomi/vim25/mo"
	"strings"
)

const (
	defaultLinux   = "rhel8"
	defaultWindows = "windows"
)


// OSFinder defines operation of discovering OS name of a VM
type OSFinder interface {
	// FindOperatingSystem tries to find operating system name of the given Vmware VM
	FindOperatingSystem(vm *mo.VirtualMachine) (string, error)
}

// VmwareOSFinder provides Vmware VM OS information
type VmwareOSFinder struct {
	OsMapProvider os.OSMapProvider
}

// FindOperatingSystem tries to find the guest operating system name of the given Vmware VM
func (r VmwareOSFinder) FindOperatingSystem(vm *mo.VirtualMachine) (string, error) {
	_, osInfoToCommon, err := r.OsMapProvider.GetOSMaps()
	if err != nil {
		return "", err
	}

	oS, found := osInfoToCommon[vm.Summary.Guest.GuestId]
	if found {
		return oS, nil
	}

	oS, found = osInfoToCommon[vm.Summary.Config.GuestId]
	if found {
		return oS, nil
	}

	// couldn't determine the exact OS from the GuestId, so at least try to determine
	// whether this is linux or windows from the guestFullName
	fullName := strings.ToLower(vm.Summary.Config.GuestFullName)
	if strings.Contains(fullName, "linux") || strings.Contains(fullName, "rhel") {
		return defaultLinux, nil
	} else if strings.Contains(fullName, "win") {
		return defaultWindows, nil
	}

	// return empty to fail label selector
	return "", fmt.Errorf("failed to find operating system for the VM")
}
