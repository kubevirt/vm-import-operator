package framework

import (
	"context"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
)

// CreateLinuxBridgeNetworkAttachmentDefinition creates Linux Bridge network attachment
func (f *Framework) CreateLinuxBridgeNetworkAttachmentDefinition() (*netv1.NetworkAttachmentDefinition, error) {
	netAttachDef := &netv1.NetworkAttachmentDefinition{}
	netAttachDef.Namespace = f.Namespace.Name
	netAttachDef.GenerateName = f.NsPrefix
	netAttachDef.Spec.Config = "{ \"cniVersion\": \"0.3.1\", \"name\": \"mynet\", \"plugins\": [{\"type\": \"bridge\", \"bridge\": \"br10\", \"vlan\": 100, \"ipam\": {}},{\"type\": \"tuning\"}]}"

	err := f.Client.Create(context.TODO(), netAttachDef)
	return netAttachDef, err
}
