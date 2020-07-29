package mappings

import (
	"context"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceFinder finds resource mappings
type ResourceFinder interface {
	GetResourceMapping(namespacedName types.NamespacedName) (*v2vv1.ResourceMapping, error)
}

// ResourceMappingsFinder provides functionality of retrieving Resource Mapping CRs
type ResourceMappingsFinder struct {
	client client.Client
}

// NewResourceMappingsFinder creates new ResourceMappingsFinder configured with given client
func NewResourceMappingsFinder(client client.Client) *ResourceMappingsFinder {
	return &ResourceMappingsFinder{
		client: client,
	}
}

// GetResourceMapping retrieves current version of a resource mapping CR with given namespaced name
func (m *ResourceMappingsFinder) GetResourceMapping(namespacedName types.NamespacedName) (*v2vv1.ResourceMapping, error) {
	instance := v2vv1.ResourceMapping{}
	err := m.client.Get(context.TODO(), namespacedName, &instance)
	if err != nil {
		return nil, err
	}
	return &instance, nil
}
