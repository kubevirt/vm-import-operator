package validators

import (
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

// TODO: move to shared package with mapping definitions

// InterfaceModelMapping defines mapping of NIC device models between oVirt and kubevirt domains
var InterfaceModelMapping = map[string]string{"e1000": "e1000", "rtl8139": "rtl8139", "virtio": "virtio", "pci_passthrough": "pci_passthrough"}

// ValidateNics validates given slice of NICs
func ValidateNics(nics []*ovirtsdk.Nic) []ValidationFailure {
	var failures []ValidationFailure
	for _, nic := range nics {
		failures = append(failures, validateNic(nic)...)
	}
	return failures
}

func validateNic(nic *ovirtsdk.Nic) []ValidationFailure {
	var results []ValidationFailure
	var nicID = ""
	if id, ok := nic.Id(); ok {
		nicID = id
	}

	if failure, valid := isValidNicInterfaceModel(nic, nicID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidNicOnBoot(nic, nicID); !valid {
		results = append(results, failure)
	}
	if failure, valid := isValidNicPlugged(nic, nicID); !valid {
		results = append(results, failure)
	}

	if vNicProfile, ok := nic.VnicProfile(); ok {
		if failure, valid := isValidVnicPortMirroring(vNicProfile, nicID); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidVnicProfileCustomProperties(vNicProfile, nicID); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidVnicProfileNetworFilter(vNicProfile, nicID); !valid {
			results = append(results, failure)
		}
		if failure, valid := isValidVnicProfileQos(vNicProfile, nicID); !valid {
			results = append(results, failure)
		}
	}

	return results
}

func isValidNicInterfaceModel(nic *ovirtsdk.Nic, nicID string) (ValidationFailure, bool) {
	iFace, ok := nic.Interface()
	if _, found := InterfaceModelMapping[string(iFace)]; !ok || !found {
		return ValidationFailure{
			ID:      NicInterfaceCheckID,
			Message: fmt.Sprintf("interface %s uses model %s that is not supported. Supported models: %v", nicID, iFace, utils.GetMapKeys(InterfaceModelMapping)),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidNicOnBoot(nic *ovirtsdk.Nic, nicID string) (ValidationFailure, bool) {
	if onBoot, _ := nic.OnBoot(); !onBoot {
		return ValidationFailure{
			ID:      NicOnBootID,
			Message: fmt.Sprintf("interface %s is not enabled on boot.", nicID),
		}, false
	}

	return ValidationFailure{}, true
}

func isValidNicPlugged(nic *ovirtsdk.Nic, nicID string) (ValidationFailure, bool) {
	if plugged, _ := nic.Plugged(); !plugged {
		return ValidationFailure{
			ID:      NicPluggedID,
			Message: fmt.Sprintf("interface %s is unplugged.", nicID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidVnicPortMirroring(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if pm, ok := vNicProfile.PortMirroring(); ok && pm {
		return ValidationFailure{
			ID:      NicVNicPortMirroringID,
			Message: fmt.Sprintf("interface %s uses profile with port mirroring.", nicID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidVnicProfileCustomProperties(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if cp, ok := vNicProfile.CustomProperties(); ok && len(cp.Slice()) > 0 {
		return ValidationFailure{
			ID:      NicVNicCustomPropertiesID,
			Message: fmt.Sprintf("interface %s uses profile with custom properties: %v.", nicID, cp),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidVnicProfileNetworFilter(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if nf, ok := vNicProfile.NetworkFilter(); ok {
		var message string
		if id, ok := nf.Id(); ok {
			message = fmt.Sprintf("Interface %s uses profile with a network filter with ID: %v.", nicID, id)
		} else {
			message = fmt.Sprintf("Interface %s uses profile with a network filter", nicID)
		}
		return ValidationFailure{
			ID:      NicVNicNetworkFilterID,
			Message: message,
		}, false
	}
	return ValidationFailure{}, true
}

func isValidVnicProfileQos(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if qos, ok := vNicProfile.Qos(); ok {
		var message string
		if id, ok := qos.Id(); ok {
			message = fmt.Sprintf("Interface %s uses profile with QOS with ID: %v.", nicID, id)
		} else {
			message = fmt.Sprintf("Interface %s uses profile with QOS", nicID)
		}
		return ValidationFailure{
			ID:      NicVNicQosID,
			Message: message,
		}, false
	}
	return ValidationFailure{}, true
}
