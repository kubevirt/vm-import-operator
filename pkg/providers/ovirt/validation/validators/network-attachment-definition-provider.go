package validators

import (
	"context"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//NetworkAttachmentDefinitions is responsible for finding network attachment definitions
type NetworkAttachmentDefinitions struct {
	Client client.Client
}

//Find retrieves network attachment definition with provided name and namespace
func (finder *NetworkAttachmentDefinitions) Find(name string, namespace string) (*netv1.NetworkAttachmentDefinition, error) {
	netAttachDef := &netv1.NetworkAttachmentDefinition{}
	err := finder.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, netAttachDef)
	return netAttachDef, err
}
