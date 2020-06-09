package utils

import (
	"strconv"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

// GetNetworkMappingName returns the network mapping name of a given network and vnic profile
func GetNetworkMappingName(networkName string, vnicProfileName string) string {
	return networkName + "/" + vnicProfileName
}

// IsCPUPinningExact checks whether CPU pinning is exact, i.e. one vCPU is pinned to exactly one CPU set
func IsCPUPinningExact(cpuTune *ovirtsdk.CpuTune) bool {
	if pins, ok := cpuTune.VcpuPins(); ok {
		var pinMapping = make(map[int]int64)
		var pinSlice = pins.Slice()
		for _, pin := range pinSlice {
			if cpuID, err := strconv.Atoi(pin.MustCpuSet()); err == nil {
				pinMapping[cpuID] = pin.MustVcpu()
			}
		}
		return len(pinMapping) == len(pinSlice)
	}
	return false
}
