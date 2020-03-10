package admission

import (
	"fmt"
	"strconv"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

// BiosTypeMapping defines mapping of BIOS types between oVirt and kubevirt domains
var BiosTypeMapping = map[string]string{"q35_ovmf": "efi", "q35_sea_bios": "bios", "q35_secure_boot": "bios"}

func validateVM(vm *ovirtsdk.Vm) []ValidationFailure {
	var results = isValidBios(vm)
	results = append(results, isValidCPU(vm)...)
	if failure, valid := isValidCPUShares(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidCustomProperties(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidDisplayType(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := hasIllegalImages(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidHighAvailabilityPriority(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidIoThreads(vm); !valid {
		results = append(results, failure)
	}
	results = append(results, isValidMemoryPolicy(vm)...)
	if failure, valid := isValidMigration(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidMigrationDowntime(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidNumaTuneMode(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidOrigin(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidRandomNumberGeneratorSource(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidSoundcardEnabled(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidStartPaused(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidTunnelMigration(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidStorageErrorResumeBehaviour(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidUsb(vm); !valid {
		results = append(results, failure)
	}
	results = append(results, isValidGraphicsConsoles(vm)...)
	if failure, valid := isValidHostDevices(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidReportedDevices(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidQuota(vm); !valid {
		results = append(results, failure)
	}
	results = append(results, isValidWatchdogs(vm)...)
	results = append(results, isValidCdroms(vm)...)
	if failure, valid := isValidFloppies(vm); !valid {
		results = append(results, failure)
	}

	return results
}

func isValidBios(vm *ovirtsdk.Vm) []ValidationFailure {
	var results []ValidationFailure
	if bios, ok := vm.Bios(); ok {
		if failure, valid := isValidBootMenu(bios); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidBiosType(bios); !valid {
			results = append(results, failure)
		}
	}
	return results
}

func isValidBootMenu(bios *ovirtsdk.Bios) (ValidationFailure, bool) {

	if bootMenu, ok := bios.BootMenu(); ok {
		if enabled, ok := bootMenu.Enabled(); ok && enabled {
			return ValidationFailure{
				ID:      VMBiosBootMenuID,
				Message: "VM Bios has boot menu enabled",
			}, false
		}
	}

	return ValidationFailure{}, true
}

func isValidBiosType(bios *ovirtsdk.Bios) (ValidationFailure, bool) {
	biosType, _ := bios.Type()
	if _, found := BiosTypeMapping[string(biosType)]; !found {
		return ValidationFailure{
			ID:      VMBiosTypeID,
			Message: fmt.Sprintf("VM uses unsupported bios type: %v", biosType),
		}, false
	}
	if biosType == "q35_secure_boot" {
		return ValidationFailure{
			ID:      VMBiosTypeQ35SecureBootID,
			Message: "VM uses q35_secure_boot bios",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidCPU(vm *ovirtsdk.Vm) []ValidationFailure {
	var results []ValidationFailure
	if cpu, ok := vm.Cpu(); ok {
		if failure, valid := isValidArchitecture(cpu); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidCPUTune(cpu); !valid {
			results = append(results, failure)
		}
	}
	return results
}

func isValidArchitecture(cpu *ovirtsdk.Cpu) (ValidationFailure, bool) {
	if arch, ok := cpu.Architecture(); ok && arch == "s390x" {
		return ValidationFailure{
			ID:      VMCpuArchitectureID,
			Message: "VM uses unsupported s390x CPU architecture",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidCPUTune(cpu *ovirtsdk.Cpu) (ValidationFailure, bool) {
	if tune, ok := cpu.CpuTune(); ok {
		if pins, ok := tune.VcpuPins(); ok {
			var pinMapping = make(map[int]int64)
			var pinSlice = pins.Slice()
			for _, pin := range pinSlice {
				if cpuID, err := strconv.Atoi(pin.MustCpuSet()); err == nil {
					pinMapping[cpuID] = pin.MustVcpu()
				}
			}
			if len(pinMapping) != len(pinSlice) {
				return ValidationFailure{
					ID:      VMCpuTuneID,
					Message: "VM uses unsupported CPU pinning layout. Only 1 vCPU - unique 1 pCpu pinning is supported",
				}, false
			}
		}
	}
	return ValidationFailure{}, true
}

func isValidCPUShares(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if _, ok := vm.CpuShares(); ok {
		return ValidationFailure{
			ID:      VMCpuSharesID,
			Message: "VM specifies CPU shares that should be available to it",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidCustomProperties(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if cp, ok := vm.CustomProperties(); ok && len(cp.Slice()) > 0 {
		return ValidationFailure{
			ID:      VMCustomPropertiesID,
			Message: fmt.Sprintf("VM specifies custom properties: %v", cp),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidDisplayType(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if display, ok := vm.Display(); ok {
		if dt, ok := display.Type(); ok && dt != "vnc" {
			return ValidationFailure{
				ID:      VMDisplayTypeID,
				Message: fmt.Sprintf("VM uses spice display: %v", display),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func hasIllegalImages(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if illegalImages, ok := vm.HasIllegalImages(); ok && illegalImages {
		return ValidationFailure{
			ID:      VMHasIllegalImagesID,
			Message: "VM has illegal images",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidHighAvailabilityPriority(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if ha, ok := vm.HighAvailability(); ok {
		if priority, ok := ha.Priority(); ok {
			return ValidationFailure{
				ID:      VMHighAvailabilityPriorityID,
				Message: fmt.Sprintf("VM uses high availability priority: %d", priority),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func isValidIoThreads(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if io, ok := vm.Io(); ok {
		if threads, ok := io.Threads(); ok {
			return ValidationFailure{
				ID:      VMIoThreadsID,
				Message: fmt.Sprintf("VM specifies IO Threads: %d", threads),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func isValidMemoryPolicy(vm *ovirtsdk.Vm) []ValidationFailure {
	var results []ValidationFailure
	if policy, ok := vm.MemoryPolicy(); ok {
		if failure, valid := isValidBallooning(policy); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidOvercommit(policy); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidGuaranteed(policy); !valid {
			results = append(results, failure)
		}
	}
	return results
}

func isValidBallooning(policy *ovirtsdk.MemoryPolicy) (ValidationFailure, bool) {
	if ballooning, ok := policy.Ballooning(); ok && ballooning {
		return ValidationFailure{
			ID:      VMMemoryPolicyBallooningID,
			Message: "VM enables memory ballooning",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidGuaranteed(policy *ovirtsdk.MemoryPolicy) (ValidationFailure, bool) {
	if guaranteed, ok := policy.Guaranteed(); ok {
		return ValidationFailure{
			ID:      VMMemoryPolicyGuaranteedID,
			Message: fmt.Sprintf("VM specifies guaranteed memory: %d", guaranteed),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidOvercommit(policy *ovirtsdk.MemoryPolicy) (ValidationFailure, bool) {
	if overcommit, ok := policy.OverCommit(); ok {
		if percent, ok := overcommit.Percent(); ok {
			return ValidationFailure{
				ID:      VMMemoryPolicyOvercommitPercentID,
				Message: fmt.Sprintf("VM specifies memory overcommit percent: %d", percent),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func isValidMigration(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if migration, ok := vm.Migration(); ok {
		return ValidationFailure{
			ID:      VMMigrationID,
			Message: fmt.Sprintf("VM has migration options specified: %v", migration),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidMigrationDowntime(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if downtime, ok := vm.MigrationDowntime(); ok {
		return ValidationFailure{
			ID:      VMMigrationDowntimeID,
			Message: fmt.Sprintf("VM has migration downtime secified: %d", downtime),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidNumaTuneMode(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if mode, ok := vm.NumaTuneMode(); ok {
		return ValidationFailure{
			ID:      VMNumaTuneModeID,
			Message: fmt.Sprintf("VM has NUMA tune mode secified: %s", mode),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidOrigin(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if origin, ok := vm.Origin(); ok && origin == "kubevirt" {
		return ValidationFailure{
			ID:      VMOriginID,
			Message: "VM has origin set to 'kubevirt'",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidRandomNumberGeneratorSource(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if rng, ok := vm.RngDevice(); ok {
		if src, ok := rng.Source(); ok && src != "urandom" {
			return ValidationFailure{
				ID:      VMRngDeviceSourceID,
				Message: fmt.Sprintf("VM has illegal random number generator device source set: %v. Supported value: 'urandom'", src),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func isValidSoundcardEnabled(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if enabled, ok := vm.SoundcardEnabled(); ok && enabled {
		return ValidationFailure{
			ID:      VMSoundcardEnabledID,
			Message: "VM has sound card enabled",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidStartPaused(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if enabled, ok := vm.StartPaused(); ok && enabled {
		return ValidationFailure{
			ID:      VMStartPausedID,
			Message: "VM has start paused enabled",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidTunnelMigration(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if enabled, ok := vm.TunnelMigration(); ok && enabled {
		return ValidationFailure{
			ID:      VMTunnelMigrationID,
			Message: "VM has start tunnel migration enabled",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidStorageErrorResumeBehaviour(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if behaviour, ok := vm.StorageErrorResumeBehaviour(); ok {
		return ValidationFailure{
			ID:      VMStorageErrorResumeBehaviourID,
			Message: fmt.Sprintf("VM has storage error resume behaviour set: %v", behaviour),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidUsb(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if usb, ok := vm.Usb(); ok {
		return ValidationFailure{
			ID:      VMUsbID,
			Message: fmt.Sprintf("VM has USB configured: %v", usb),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidGraphicsConsoles(vm *ovirtsdk.Vm) []ValidationFailure {
	var results []ValidationFailure
	if gfx, ok := vm.GraphicsConsoles(); ok {
		for _, console := range gfx.Slice() {
			if protocol, ok := console.Protocol(); ok && protocol != "vnc" {
				results = append(results, ValidationFailure{
					ID:      VMGraphicConsolesID,
					Message: fmt.Sprintf("VM has non-VNC graphics console configured: %v", console),
				})
			}
		}
	}
	return results
}

func isValidHostDevices(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if devices, ok := vm.HostDevices(); ok && len(devices.Slice()) > 0 {
		return ValidationFailure{
			ID:      VMHostDevicesID,
			Message: fmt.Sprintf("VM has following host devices: %v", devices),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidReportedDevices(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if devices, ok := vm.ReportedDevices(); ok && len(devices.Slice()) > 0 {
		return ValidationFailure{
			ID:      VMReportedDevicesID,
			Message: fmt.Sprintf("VM has following reported devices: %v", devices),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidQuota(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if quota, ok := vm.Quota(); ok {
		return ValidationFailure{
			ID:      VMQuotaID,
			Message: fmt.Sprintf("VM has quota assigned: %v", quota),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidWatchdogs(vm *ovirtsdk.Vm) []ValidationFailure {
	var results []ValidationFailure
	if watchdogs, ok := vm.Watchdogs(); ok {
		for _, wd := range watchdogs.Slice() {
			if model, ok := wd.Model(); ok && model != "i6300esb" {
				results = append(results, ValidationFailure{
					ID:      VMWatchdogsID,
					Message: fmt.Sprintf("VM has unsupported watchdog configured: %v", wd),
				})
			}
		}
	}
	return results
}

func isValidCdroms(vm *ovirtsdk.Vm) []ValidationFailure {
	var results []ValidationFailure
	if cdroms, ok := vm.Cdroms(); ok {
		for _, cdrom := range cdroms.Slice() {
			if file, ok := cdrom.File(); ok {
				if storageType, ok := getStorageDomainType(file); ok && storageType != "data" {
					results = append(results, ValidationFailure{
						ID:      VMCdromsID,
						Message: fmt.Sprintf("VM uses CD ROM with image not stored in data domain: %v", cdrom),
					})
				}
			}
		}
	}
	return results
}

func getStorageDomainType(file *ovirtsdk.File) (string, bool) {
	if sd, ok := file.StorageDomain(); ok {
		if sdType, ok := sd.Type(); ok {
			return string(sdType), true
		}
	}
	return "", false
}

func isValidFloppies(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if floppies, ok := vm.Floppies(); ok && len(floppies.Slice()) > 0 {
		return ValidationFailure{
			ID:      VMFloppiesID,
			Message: fmt.Sprintf("VM uses floppies: %v", floppies),
		}, false
	}
	return ValidationFailure{}, true
}
