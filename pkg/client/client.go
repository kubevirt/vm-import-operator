package client

// Factory creates new clients
type Factory interface {
	NewOvirtClient(dataMap map[string]string) (VMClient, error)
}

// VMClient provides interface how source virtual machines should be fetched
type VMClient interface {
	GetVM(id *string, name *string, cluster *string, clusterID *string) (interface{}, error)
	StopVM(id string) error
	StartVM(id string) error
	Close() error
}
