package admission

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

const (
	// NicInterfaceCheckID defines an ID of a NIC interface model check
	NicInterfaceCheckID = CheckID("nic.interface")
	// NicOnBootID defines an ID of a NIC on_boot == fales check
	NicOnBootID = CheckID("nic.on_boot")
	// NicPluggedID defines an ID of a NIC plugged == false check
	NicPluggedID = CheckID("nic.plugged")
	// NicVNicPortMirroringID defines an ID of a vnic_profile.port_mirroring == true check
	NicVNicPortMirroringID = CheckID("nic.vnic_profile.port_mirroring")
	// NicVNicPassThroughID defines an ID of a vnic_profile.pass_through == 'enabled' check
	NicVNicPassThroughID = CheckID("nic.vnic_profile.pass_through")
	// NicVNicCustomPropertiesID defines an ID of a vnic_profile.custom_properties presence check
	NicVNicCustomPropertiesID = CheckID("nic.vnic_profile.custom_properties")
	// NicVNicNetworkFilterID defines an ID of a vnic_profile.networ_filter presence check
	NicVNicNetworkFilterID = CheckID("nic.vnic_profile.network_filter")
	// NicVNicQosID defines an ID of a vnic_profile.qos presence check
	NicVNicQosID = CheckID("nic.vnic_profile.qos")
)

var validInterfaceDeviceModels = map[string]*struct{}{"e1000": nil, "rtl8139": nil, "virtio": nil}

// CheckID identifies validation check for Virtual Machine Import
type CheckID string

// ValidationFailure describes Virtual Machine Import validation failure
type ValidationFailure struct {
	// Check ID
	ID CheckID
	// Verbose explanation of the failure
	Message string
}

func validateNics(nics []*ovirtsdk.Nic) []ValidationFailure {
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

	//TODO: Networking rule #1

	if failure, valid := isValidatNicInterfaceModel(nic, nicID); !valid {
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
		if failure, valid := isValidVnicProfileMigratable(vNicProfile, nicID); !valid {
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

func isValidatNicInterfaceModel(nic *ovirtsdk.Nic, nicID string) (ValidationFailure, bool) {
	iFace, ok := nic.Interface()
	if _, found := validInterfaceDeviceModels[string(iFace)]; !ok || !found {
		return ValidationFailure{
			ID:      NicInterfaceCheckID,
			Message: fmt.Sprintf("interface %v uses model %s that is not supported.", nicID, iFace),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidNicOnBoot(nic *ovirtsdk.Nic, nicID string) (ValidationFailure, bool) {
	if onBoot, _ := nic.OnBoot(); !onBoot {
		return ValidationFailure{
			ID:      NicOnBootID,
			Message: fmt.Sprintf("interface %v is not enabled on boot.", nicID),
		}, false
	}

	return ValidationFailure{}, true
}

func isValidNicPlugged(nic *ovirtsdk.Nic, nicID string) (ValidationFailure, bool) {
	if plugged, _ := nic.Plugged(); !plugged {
		return ValidationFailure{
			ID:      NicPluggedID,
			Message: fmt.Sprintf("interface %v is unplugged.", nicID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidVnicPortMirroring(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if pm, ok := vNicProfile.PortMirroring(); ok && pm {
		return ValidationFailure{
			ID:      NicVNicPortMirroringID,
			Message: fmt.Sprintf("interface %v uses profile with port mirroring.", nicID),
		}, false
	}
	return ValidationFailure{}, true
}

func isValidVnicProfileMigratable(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if pt, ok := vNicProfile.PassThrough(); ok {
		if ptm, ok := pt.Mode(); ok && string(ptm) == "enabled" {
			return ValidationFailure{
				ID:      NicVNicPassThroughID,
				Message: fmt.Sprintf("interface %v uses profile pass-through enabled.", nicID),
			}, false
		}
	}
	return ValidationFailure{}, true
}

func isValidVnicProfileCustomProperties(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if cp, ok := vNicProfile.CustomProperties(); ok && len(cp.Slice()) > 0 {
		return ValidationFailure{
			ID:      NicVNicCustomPropertiesID,
			Message: fmt.Sprintf("interface %v uses profile with custom properties: %v.", nicID, cp),
		}, false
	}
	return ValidationFailure{}, true
}
func isValidVnicProfileNetworFilter(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if nf, ok := vNicProfile.NetworkFilter(); ok {
		return ValidationFailure{
			ID:      NicVNicNetworkFilterID,
			Message: fmt.Sprintf("interface %v uses profile with a network filter: %v.", nicID, nf),
		}, false
	}
	return ValidationFailure{}, true
}
func isValidVnicProfileQos(vNicProfile *ovirtsdk.VnicProfile, nicID string) (ValidationFailure, bool) {
	if qos, ok := vNicProfile.Qos(); ok {
		return ValidationFailure{
			ID:      NicVNicQosID,
			Message: fmt.Sprintf("interface %v uses profile with QOS: %v.", nicID, qos),
		}, false
	}
	return ValidationFailure{}, true
}
