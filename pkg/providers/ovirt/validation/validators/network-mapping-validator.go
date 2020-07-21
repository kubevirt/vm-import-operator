package validators

import (
	"fmt"
	"strings"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	outils "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/utils"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

//NetworkAttachmentDefinitionProvider provides NetworkAttachmentDefinition retrieval capabilities
type NetworkAttachmentDefinitionProvider interface {
	//Find retrieves NetworkAttachmentDefinition for given name and optional namespace
	Find(name string, namespace string) (*netv1.NetworkAttachmentDefinition, error)
}

//NetworkMappingValidator provides network mappings validation logic
type NetworkMappingValidator struct {
	provider NetworkAttachmentDefinitionProvider
}

//NewNetworkMappingValidator creates new NetworkMappingValidator that will use given provider
func NewNetworkMappingValidator(provider NetworkAttachmentDefinitionProvider) NetworkMappingValidator {
	return NetworkMappingValidator{
		provider: provider,
	}
}

//ValidateNetworkMapping validates network mapping
func (v *NetworkMappingValidator) ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1alpha1.NetworkResourceMappingItem, crNamespace string) []ValidationFailure {
	var failures []ValidationFailure
	// Check whether mapping for network is required and was provided
	if mapping == nil {
		if v.hasAtLeastOneWithVNicProfile(nics) {
			failures = append(failures, ValidationFailure{
				ID:      NetworkMappingID,
				Message: "Network mapping is missing",
			})
		}
		return failures
	}

	// Map source id and name to ResourceMappingItem
	mapByID, mapByName := utils.IndexNetworkByIDAndName(mapping)

	// validate source network format comply to network-name/vnic-profile-name
	failure, ok := v.validateSourceNetworkFormat(mapByName)
	if !ok {
		failures = append(failures, failure)
		return failures
	}

	// Get all vnic profiles needed by the VM as slice of sources
	requiredVnicProfiles := v.getRequiredVnicProfiles(nics)

	requiredTargetsSet := make(map[v2vv1alpha1.ObjectIdentifier]*string)
	// Validate that all vm networks are mapped and populate requiredTargetsSet for target existence check
	for _, vnic := range requiredVnicProfiles {
		if vnic.ID != nil {
			item, found := mapByID[*vnic.ID]
			if found {
				requiredTargetsSet[item.Target] = item.Type
				continue
			}
		}
		if vnic.Name != nil {
			item, found := mapByName[*vnic.Name]
			if found {
				requiredTargetsSet[item.Target] = item.Type
				continue
			}
		}
		failures = append(failures, ValidationFailure{
			ID:      NetworkMappingID,
			Message: fmt.Sprintf("Required source Vnic Profile '%s' lacks mapping", utils.ToLoggableID(vnic.ID, vnic.Name)),
		})
	}

	podNetworks := v.getPodNetworks(nics, mapByID, mapByName)
	if len(podNetworks) > 1 {
		failures = append(failures, ValidationFailure{
			ID:      NetworkMultiplePodTargetsID,
			Message: fmt.Sprintf("There are more than one source networks mapped to a pod network: %v ", podNetworks),
		})
	}

	// Validate that all target networks needed by the VM exist in k8s
	for networkID, networkType := range requiredTargetsSet {
		if failure, valid := v.validateNetwork(networkID, networkType, crNamespace); !valid {
			failures = append(failures, failure)
		}
	}
	return failures
}

func (v *NetworkMappingValidator) getPodNetworks(nics []*ovirtsdk.Nic, mapByID map[string]v2vv1alpha1.NetworkResourceMappingItem, mapByName map[string]v2vv1alpha1.NetworkResourceMappingItem) []string {
	var podNetworks []string
	for _, nic := range nics {
		if vnicProfile, ok := nic.VnicProfile(); ok {
			if id, ok := vnicProfile.Id(); ok {
				if item, found := mapByID[id]; found {
					if item.Type == nil || *item.Type == "pod" {
						podNetworks = append(podNetworks, utils.ToLoggableID(item.Source.ID, item.Source.Name))
						continue
					}
				}
			}
			if name, found := vnicProfile.Name(); found {
				if item, ok := mapByName[name]; ok {
					if item.Type == nil || *item.Type == "pod" {
						podNetworks = append(podNetworks, utils.ToLoggableID(item.Source.ID, item.Source.Name))
					}
				}
			}
		}
	}
	return podNetworks
}

func (v *NetworkMappingValidator) hasAtLeastOneWithVNicProfile(nics []*ovirtsdk.Nic) bool {
	for _, nic := range nics {
		if _, ok := nic.VnicProfile(); ok {
			return true
		}
	}
	return false
}

func (v *NetworkMappingValidator) validateSourceNetworkFormat(mapByName map[string]v2vv1alpha1.NetworkResourceMappingItem) (ValidationFailure, bool) {
	invalidNames := make([]string, 0)
	for k := range mapByName {
		if !strings.Contains(k, "/") {
			invalidNames = append(invalidNames, k)
		}
	}
	if len(invalidNames) > 0 {
		message := fmt.Sprintf("Network mapping name format is invalid: %v. Expected format is 'network-name/vnic-profile-name'", invalidNames)
		return ValidationFailure{
			ID:      NetworkMappingID,
			Message: message,
		}, false
	}

	return ValidationFailure{}, true
}

func (v *NetworkMappingValidator) validateNetwork(networkID v2vv1alpha1.ObjectIdentifier, networkType *string, crNamespace string) (ValidationFailure, bool) {
	if networkType == nil {
		if networkID.Namespace == nil {
			// unspecified network type is only valid when target network doesn't specify namespace
			return ValidationFailure{}, true
		}
		return ValidationFailure{
			ID:      NetworkTypeID,
			Message: fmt.Sprintf("Network %s has unspecified network type and target namespace: %s. When target type is omitted, the namespace must be omitted as well.", utils.ToLoggableResourceName(networkID.Name, networkID.Namespace), *networkID.Namespace),
		}, false
	}

	switch *networkType {
	case "pod":
		return ValidationFailure{}, true
	case "multus":
		return v.isValidMultusNetwork(networkID, crNamespace)
	default:
		return ValidationFailure{
			ID:      NetworkTypeID,
			Message: fmt.Sprintf("Network %s has unsupported network type: %v", utils.ToLoggableResourceName(networkID.Name, networkID.Namespace), networkType),
		}, false
	}
}

func (v *NetworkMappingValidator) isValidMultusNetwork(networkID v2vv1alpha1.ObjectIdentifier, crNamespace string) (ValidationFailure, bool) {
	namespace := crNamespace
	if networkID.Namespace != nil {
		namespace = *networkID.Namespace
	}
	_, err := v.provider.Find(networkID.Name, namespace)
	if err != nil {
		return ValidationFailure{
			ID:      NetworkTargetID,
			Message: fmt.Sprintf("Network Attachment Defintion %s has not been found. Error: %v", utils.ToLoggableResourceName(networkID.Name, &namespace), err),
		}, false
	}

	return ValidationFailure{}, true
}

func (v *NetworkMappingValidator) getRequiredVnicProfiles(nics []*ovirtsdk.Nic) []v2vv1alpha1.Source {
	sourcesSet := make(map[v2vv1alpha1.Source]bool)
	for _, nic := range nics {
		if vnic, ok := nic.VnicProfile(); ok {
			if network, ok := vnic.Network(); ok {
				if src, ok := v.createSourceNetworkIdentifier(network, vnic); ok {
					sourcesSet[*src] = true
				}
			}
		}
	}
	var sources []v2vv1alpha1.Source
	for source := range sourcesSet {
		sources = append(sources, source)
	}
	return sources
}

func (v *NetworkMappingValidator) createSourceNetworkIdentifier(network *ovirtsdk.Network, vnic *ovirtsdk.VnicProfile) (*v2vv1alpha1.Source, bool) {
	id, okID := vnic.Id()
	networkName, okNetworkName := network.Name()
	vnicName, okVnicName := vnic.Name()
	if okID || okNetworkName && okVnicName {
		name := outils.GetNetworkMappingName(networkName, vnicName)
		src := v2vv1alpha1.Source{
			ID:   &id,
			Name: &name}
		return &src, true
	}
	return nil, false
}
