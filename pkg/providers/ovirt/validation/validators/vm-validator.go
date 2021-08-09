package validators

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"

	"github.com/kubevirt/vm-import-operator/pkg/utils"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
	otemplates "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/templates"
	outils "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

type rule struct {
	Name string      `json:"name"`
	Min  interface{} `json:"min,omitempty"`
}

// ValidateVM validates given VM
func ValidateVM(vm *ovirtsdk.Vm, config kvConfig.KubeVirtConfig, finder *otemplates.TemplateFinder) []ValidationFailure {
	var results = isValidBios(vm)
	if failure, valid := isValidStatus(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidTimezone(vm); !valid {
		results = append(results, failure)
	}
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
	if failure, valid := isValidCustomEmulatedMachine(vm); !valid {
		results = append(results, failure)
	}
	if failure, valid := isMemoryAboveRequests(vm, finder); !valid {
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
		var clusterDefaultBiosType *ovirtsdk.BiosType
		if cluster, ok := vm.Cluster(); ok {
			if biosType, ok := cluster.BiosType(); ok {
				clusterDefaultBiosType = &biosType
			}
		}
		if failure, valid := isValidBiosType(bios, clusterDefaultBiosType); !valid {
			results = append(results, failure)
		}
	}
	return results
}

func isValidCustomEmulatedMachine(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if emulatedMachine, ok := vm.CustomEmulatedMachine(); ok {
		if strings.Contains(emulatedMachine, "i440fx") {
			return ValidationFailure{
				ID:      VMCustomEmulatedMachine,
				Message: fmt.Sprintf("Source VM use custom emulated machine which is overwitten by cpu architecture."),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func isMemoryAboveRequests(vm *ovirtsdk.Vm, finder *otemplates.TemplateFinder) (ValidationFailure, bool) {
	template, err := finder.FindTemplate(vm)
	if err != nil {
		// missing template is verified later
		return ValidationFailure{}, true
	}
	validations := template.Annotations["validations"]
	var rules []rule
	err = json.Unmarshal([]byte(validations), &rules)
	if err != nil {
		// ignoring, issue with template validation rules
		return ValidationFailure{}, true
	}

	for i := range rules {
		r := &rules[i]
		if r.Name == "minimal-required-memory" {
			sourceMem, sok := vm.Memory()
			tempMem, ok := toInt64(r.Min)
			if ok && sok && tempMem > sourceMem {
				return ValidationFailure{
					ID:      VMMemoryTemplateLimitID,
					Message: fmt.Sprintf("Source VM memory %d is lower than %d enforced by %s template.", sourceMem, tempMem, template.Name),
				}, false
			}
		}
	}

	return ValidationFailure{}, true
}

func toInt64(obj interface{}) (int64, bool) {
	switch val := obj.(type) {
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return int64(val), true
	case uint:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint64:
		return int64(val), true
	case float32:
		return int64(math.Round(float64(val))), true
	case float64:
		return int64(math.Round(val)), true
	}
	return 0, false
}

func isValidStatus(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if status, ok := vm.Status(); ok {
		if status != ovirtsdk.VMSTATUS_UP && status != ovirtsdk.VMSTATUS_DOWN {
			return ValidationFailure{
				ID:      VMStatusID,
				Message: fmt.Sprintf("VM has illegal status: %v. Only 'up' and 'down' are allowed.", status),
			}, false
		}
		return ValidationFailure{}, true
	}
	return ValidationFailure{
		ID:      VMStatusID,
		Message: "VM doesn't have any status. Must be 'up' or 'down'.",
	}, false
}

func isValidTimezone(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if tz, ok := vm.TimeZone(); ok {
		if name, ok := tz.Name(); ok {
			if !utils.IsUtcCompatible(name) {
				return ValidationFailure{
					ID:      VMTimezoneID,
					Message: "VM's timezone is not UTC-compatible. It should have offset of 0 and not observe DST",
				}, false
			}
		}
	}
	return ValidationFailure{}, true
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

func isValidBiosType(bios *ovirtsdk.Bios, clusterDefaultBiosType *ovirtsdk.BiosType) (ValidationFailure, bool) {
	biosType, _ := bios.Type()
	if biosType == "cluster_default" && clusterDefaultBiosType != nil {
		biosType = *clusterDefaultBiosType
	}
	if _, found := mapper.BiosTypeMapping[string(biosType)]; !found {
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
		if !outils.IsCPUPinningExact(tune) {
			return ValidationFailure{
				ID:      VMCpuTuneID,
				Message: "VM uses unsupported CPU pinning layout. Only 1 vCPU - unique 1 pCpu pinning is supported",
			}, false
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
				Message: "VM uses spice display",
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
	if _, ok := vm.Migration(); ok {
		return ValidationFailure{
			ID:      VMMigrationID,
			Message: "VM has migration options specified",
		}, false
	}
	return ValidationFailure{}, true
}

func isValidMigrationDowntime(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if downtime, ok := vm.MigrationDowntime(); ok {
		return ValidationFailure{
			ID:      VMMigrationDowntimeID,
			Message: fmt.Sprintf("VM has migration downtime specified: %d", downtime),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidNumaTuneMode(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if mode, ok := vm.NumaTuneMode(); ok {
		return ValidationFailure{
			ID:      VMNumaTuneModeID,
			Message: fmt.Sprintf("VM has NUMA tune mode specified: %s", mode),
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

func isValidPlacementPolicy(vm *ovirtsdk.Vm, liveMigrationEnabled bool) (ValidationFailure, bool) {
	if pp, ok := vm.PlacementPolicy(); ok {
		if affinity, _ := pp.Affinity(); affinity == ovirtsdk.VMAFFINITY_MIGRATABLE {
			if !liveMigrationEnabled {
				return ValidationFailure{
					ID:      VMPlacementPolicyAffinityID,
					Message: "VM has placement policy affinity set to `migratable`",
				}, false
			}
		}
	}
	return ValidationFailure{}, true
}

func isValidRandomNumberGeneratorSource(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if rng, ok := vm.RngDevice(); ok {
		if src, ok := rng.Source(); ok && src != "urandom" {
			return ValidationFailure{
				ID:      VMRngDeviceSourceID,
				Message: fmt.Sprintf("VM has unsupported random number generator device source set: %v. Supported value: 'urandom'", src),
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
		if enabled, ok := usb.Enabled(); ok && enabled {
			return ValidationFailure{
				ID:      VMUsbID,
				Message: "VM has USB enabled",
			}, false
		}
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
					Message: fmt.Sprintf("VM has non-VNC graphics console configured: %v", protocol),
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
			Message: fmt.Sprintf("VM has following reported devices: %v", reportedDevicesToStrings(devices.Slice())),
		}, false
	}
	return ValidationFailure{}, true
}

func reportedDevicesToStrings(devices []*ovirtsdk.ReportedDevice) []string {
	var strings []string
	for _, rd := range devices {
		strings = append(strings, reportedDeviceToString(rd))
	}
	return strings
}

func reportedDeviceToString(rd *ovirtsdk.ReportedDevice) string {
	var id, name *string
	if rdID, ok := rd.Id(); ok {
		id = &rdID
	}
	if rdName, ok := rd.Name(); ok {
		name = &rdName
	}
	return utils.ToLoggableID(id, name)
}

func isValidQuota(vm *ovirtsdk.Vm) (ValidationFailure, bool) {
	if quota, ok := vm.Quota(); ok {
		var message string
		if id, ok := quota.Id(); ok {
			message = fmt.Sprintf("VM has quota with ID %v assigned", id)
		} else {
			message = "VM has quota assigned"
		}
		return ValidationFailure{
			ID:      VMQuotaID,
			Message: message,
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
					var message string
					if id, ok := cdrom.Id(); ok {
						message = fmt.Sprintf("VM uses CD ROM with image not stored in data domain: %v", id)
					} else {
						message = "VM uses CD ROM with image not stored in data domain"
					}
					results = append(results, ValidationFailure{
						ID:      VMCdromsID,
						Message: message,
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
			Message: fmt.Sprintf("VM uses %d floppies", len(floppies.Slice())),
		}, false
	}
	return ValidationFailure{}, true
}
