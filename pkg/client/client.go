package client

import (
	ovirtclient "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/client"
	vmwareclient "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/client"
)

// Factory creates new clients
type Factory interface {
	NewOvirtClient(dataMap map[string]string) (VMClient, error)
	NewVmwareClient(dataMap map[string]string) (VMClient, error)
}

// VMClient provides interface how source virtual machines should be fetched
type VMClient interface {
	TestConnection() error
	GetVM(id *string, name *string, cluster *string, clusterID *string) (interface{}, error)
	StopVM(id string) error
	StartVM(id string) error
	Close() error
}

// SourceClientFactory provides default client factory implementation
type SourceClientFactory struct{}

// NewSourceClientFactory creates new factory
func NewSourceClientFactory() *SourceClientFactory {
	return &SourceClientFactory{}
}

// NewOvirtClient creates new Ovirt clients
func (f *SourceClientFactory) NewOvirtClient(dataMap map[string]string) (VMClient, error) {
	return ovirtclient.NewRichOvirtClient(&ovirtclient.ConnectionSettings{
		URL:      dataMap["apiUrl"],
		Username: dataMap["username"],
		Password: dataMap["password"],
		CACert:   []byte(dataMap["caCert"]),
	})
}

// NewVmwareClient creates new VMWare clients
func (f *SourceClientFactory) NewVmwareClient(dataMap map[string]string) (VMClient, error) {
	return vmwareclient.NewRichVMWareClient(
		dataMap["apiUrl"],
		dataMap["username"],
		dataMap["password"],
		dataMap["thumbprint"])
}
