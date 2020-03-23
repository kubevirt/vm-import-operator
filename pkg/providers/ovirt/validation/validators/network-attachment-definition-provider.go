package validators

import (
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkclient "kubevirt.io/client-go/generated/network-attachment-definition-client/clientset/versioned"
)

//NetworkAttachmentDefinitions is responsible for finding network attachment definitions
type NetworkAttachmentDefinitions struct {
	Client networkclient.Interface
}

//Find retrieves network attachment definition with provided name and namespace
func (finder *NetworkAttachmentDefinitions) Find(name string, namespace string) (*netv1.NetworkAttachmentDefinition, error) {
	return finder.Client.K8sCniCncfIoV1().
		NetworkAttachmentDefinitions(namespace).
		Get(name, metav1.GetOptions{})
}
