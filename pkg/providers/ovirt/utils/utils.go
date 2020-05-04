package utils

// GetNetworkMappingName returns the network mapping name of a given network and vnic profile
func GetNetworkMappingName(networkName string, vnicProfileName string) string {
	return networkName + "/" + vnicProfileName
}
