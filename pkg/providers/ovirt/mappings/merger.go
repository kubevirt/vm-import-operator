package mappings

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
)

// MergeMappings creates new resource mapping spec containing all of the mappings from the externalMappingSpec with all mappings from crMappings. Mappings from the crMappings will overwrite ones from the external mapping.
func MergeMappings(externalMappingSpec *v2vv1.ResourceMappingSpec, vmiMapping *v2vv1.OvirtMappings) *v2vv1.OvirtMappings {
	if externalMappingSpec == nil && vmiMapping == nil {
		return &v2vv1.OvirtMappings{}
	}
	primaryMappings, secondaryMappings := extractMappings(externalMappingSpec, vmiMapping)

	networkMappings := mappings.MergeNetworkMappings(primaryMappings.NetworkMappings, secondaryMappings.NetworkMappings)
	storageMappings := mappings.MergeStorageMappings(primaryMappings.StorageMappings, secondaryMappings.StorageMappings)

	// diskMappings are expected to be provided only for a specific VM Import CR
	diskMappings := primaryMappings.DiskMappings

	ovirtMappings := v2vv1.OvirtMappings{
		NetworkMappings: networkMappings,
		StorageMappings: storageMappings,
		DiskMappings:    diskMappings,
	}
	return &ovirtMappings
}

func extractMappings(externalMappingSpec *v2vv1.ResourceMappingSpec, crMappings *v2vv1.OvirtMappings) (*v2vv1.OvirtMappings, *v2vv1.OvirtMappings) {
	var primaryMappings, secondaryMappings v2vv1.OvirtMappings
	if crMappings != nil {
		primaryMappings = *crMappings
	}

	if externalMappingSpec != nil && externalMappingSpec.OvirtMappings != nil {
		secondaryMappings = *externalMappingSpec.OvirtMappings
	}
	return &primaryMappings, &secondaryMappings
}
