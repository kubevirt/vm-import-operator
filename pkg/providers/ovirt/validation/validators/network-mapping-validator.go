package validators

import (
	"fmt"

	"kubevirt.io/client-go/kubecli"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

//NetworkAttachmentDefinitionProvider retrieves NetworkAttachmentDefinition for given name and optional namespace
type NetworkAttachmentDefinitionProvider interface {
	Find(name string, namespace string) (*netv1.NetworkAttachmentDefinition, error)
}

//NetworkMappingValidator provides network mappings validation logic
type NetworkMappingValidator struct {
	provider NetworkAttachmentDefinitionProvider
}

//NewNetworkMappingValidator creates new NetworkMappingValidator that will use provided KubevirtClient
func NewNetworkMappingValidator(kubevirt kubecli.KubevirtClient) NetworkMappingValidator {
	return NetworkMappingValidator{
		provider: &NetworkAttachmentDefinitions{
			Client: kubevirt.NetworkClient(),
		},
	}
}

//ValidateNetworkMapping validates network mapping
func (v *NetworkMappingValidator) ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1alpha1.ResourceMappingItem, defaultNamespace string) []ValidationFailure {
	var failures []ValidationFailure
	// Check whether mapping for network is required and was provided
	if mapping == nil && len(nics) > 0 {
		failures = append(failures, ValidationFailure{
			ID:      NetworkMappingID,
			Message: "Network mapping is missing",
		})
		return failures
	}
	// Get all networks needed by the VM as slice of sources
	requiredNetworks := getRequiredNetworks(nics)
	mapByID := make(map[string]v2vv1alpha1.ResourceMappingItem)
	mapByName := make(map[string]v2vv1alpha1.ResourceMappingItem)
	// Map source id and name to ResourceMappingItem
	for _, item := range *mapping {
		if item.Source.ID != nil {
			mapByID[*item.Source.ID] = item
		}
		if item.Source.Name != nil {
			mapByName[*item.Source.Name] = item
		}
	}
	requiredTargetsSet := make(map[v2vv1alpha1.ObjectIdentifier]*string)
	// Validate that all vm networks are mapped and populate requiredTargetsSet for target existence check
	for _, network := range requiredNetworks {
		if network.ID != nil {
			item, found := mapByID[*network.ID]
			if found {
				requiredTargetsSet[item.Target] = item.Type
				continue
			}
		}
		if network.Name != nil {
			item, found := mapByName[*network.Name]
			if found {
				requiredTargetsSet[item.Target] = item.Type
				continue
			}
		}
		failures = append(failures, ValidationFailure{
			ID:      NetworkMappingID,
			Message: fmt.Sprintf("Required source network '%s' lacks mapping", ToLoggableID(network.ID, network.Name)),
		})
	}

	// Validate that all target networks needed by the VM exist in k8s
	for networkID, networkType := range requiredTargetsSet {
		if networkType == nil {
			continue
		}
		if failure, valid := v.validateNetwork(networkID, *networkType, defaultNamespace); !valid {
			failures = append(failures, failure)
		}
	}
	return failures
}

func (v *NetworkMappingValidator) validateNetwork(networkID v2vv1alpha1.ObjectIdentifier, networkType string, defaultNamespace string) (ValidationFailure, bool) {
	switch networkType {
	case "pod":
		return ValidationFailure{}, true
	case "multus":
		return v.isValidMultusNetwork(networkID, defaultNamespace)
	default:
		return ValidationFailure{
			ID:      NetworkTypeID,
			Message: fmt.Sprintf("Network %s has unsupported network type: %s", ToLoggableResourceName(networkID.Name, networkID.Namespace), networkType),
		}, false
	}
}

func (v *NetworkMappingValidator) isValidMultusNetwork(networkID v2vv1alpha1.ObjectIdentifier, defaultNamespace string) (ValidationFailure, bool) {
	namespace := defaultNamespace
	if networkID.Namespace != nil {
		namespace = *networkID.Namespace
	}
	_, err := v.provider.Find(networkID.Name, namespace)
	if err != nil {
		return ValidationFailure{
			ID:      NetworkTargetID,
			Message: fmt.Sprintf("Network %s/%s has not been found. Error: %v", namespace, networkID.Name, err),
		}, false
	}

	return ValidationFailure{}, true
}

func getRequiredNetworks(nics []*ovirtsdk.Nic) []v2vv1alpha1.Source {
	sourcesSet := make(map[v2vv1alpha1.Source]bool)
	for _, nic := range nics {
		if network, ok := nic.Network(); ok {
			if src, ok := maybeCreateSource(network); ok {
				sourcesSet[*src] = true
			}
		}
		if vnic, ok := nic.VnicProfile(); ok {
			if network, ok := vnic.Network(); ok {
				if src, ok := maybeCreateSource(network); ok {
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

func maybeCreateSource(network *ovirtsdk.Network) (*v2vv1alpha1.Source, bool) {
	id, okID := network.Id()
	name, okName := network.Name()
	if okID || okName {
		src := v2vv1alpha1.Source{
			ID:   &id,
			Name: &name}
		return &src, true
	}
	return nil, false
}
