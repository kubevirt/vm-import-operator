package os

import (
	"fmt"
	"strings"

	"github.com/kubevirt/vm-import-operator/pkg/os"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

const (
	defaultLinux   = "rhel8.2"
	defaultWindows = "win10"
)

// OSFinder defines operation of discovering OS name of a VM
type OSFinder interface {
	// FindOperatingSystem tries to find operating system name of the given oVirt VM
	FindOperatingSystem(*ovirtsdk.Vm) (string, error)
}

// OVirtOSFinder provides oVirt VM OS information
type OVirtOSFinder struct {
	OsMapProvider os.OSMapProvider
}

// FindOperatingSystem tries to find operating system name of the given oVirt VM
func (o *OVirtOSFinder) FindOperatingSystem(vm *ovirtsdk.Vm) (string, error) {
	guestOsToCommon, osInfoToCommon, err := o.OsMapProvider.GetOSMaps()
	if err != nil {
		return "", err
	}
	// Attempt resolving OS based on VM Guest OS information
	if gos, found := vm.GuestOperatingSystem(); found {
		distribution, _ := gos.Distribution()
		version, _ := gos.Version()
		fullVersion, _ := version.FullVersion()
		os, found := guestOsToCommon[distribution]
		if found {
			return fmt.Sprintf("%s%s", os, fullVersion), nil
		}
	}
	// Attempt resolving OS by looking for a match based on OS mapping
	if os, found := vm.Os(); found {
		osType, _ := os.Type()
		mappedOS, found := osInfoToCommon[osType]
		if found {
			return mappedOS, nil
		}

		// limit number of possibilities
		osType = strings.ToLower(osType)
		if strings.Contains(osType, "linux") || strings.Contains(osType, "rhel") {
			return defaultLinux, nil
		} else if strings.Contains(osType, "win") {
			return defaultWindows, nil
		}
	}
	// return empty to fail label selector
	return "", fmt.Errorf("Failed to find operating system for the VM")
}
