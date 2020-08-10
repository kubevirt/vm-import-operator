package mappings

import (
	"github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
)

// MergeMappings creates new resource mapping spec containing all of the mappings from the externalMappingSpec with all mappings from crMappings. Mappings from the crMappings will overwrite ones from the external mapping.
func MergeMappings(externalMappingSpec *v1beta1.ResourceMappingSpec, vmiMapping *v1beta1.VmwareMappings) *v1beta1.VmwareMappings {
	if externalMappingSpec == nil && vmiMapping == nil {
		return &v1beta1.VmwareMappings{}
	}
	primaryMappings, secondaryMappings := extractMappings(externalMappingSpec, vmiMapping)

	networkMappings := mappings.MergeNetworkMappings(primaryMappings.NetworkMappings, secondaryMappings.NetworkMappings)
	storageMappings := mappings.MergeStorageMappings(primaryMappings.StorageMappings, secondaryMappings.StorageMappings)

	// diskMappings are expected to be provided only for a specific VM Import CR
	diskMappings := primaryMappings.DiskMappings

	vmwareMappings := v1beta1.VmwareMappings{
		DiskMappings:    diskMappings,
		NetworkMappings: networkMappings,
		StorageMappings: storageMappings,
	}
	return &vmwareMappings
}

func extractMappings(externalMappingSpec *v1beta1.ResourceMappingSpec, crMappings *v1beta1.VmwareMappings) (*v1beta1.VmwareMappings, *v1beta1.VmwareMappings) {
	var primaryMappings, secondaryMappings v1beta1.VmwareMappings
	if crMappings != nil {
		primaryMappings = *crMappings
	}

	if externalMappingSpec != nil && externalMappingSpec.VmwareMappings != nil {
		secondaryMappings = *externalMappingSpec.VmwareMappings
	}
	return &primaryMappings, &secondaryMappings
}
