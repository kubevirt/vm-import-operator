package kubevirt

import (
	config "github.com/kubevirt/vm-import-operator/pkg/config"
	"k8s.io/client-go/kubernetes"
)

const (
	configMapName = "kubevirt-config"
)

// KubeVirtConfigProvider defines KubeVirt config access operations
type KubeVirtConfigProvider interface {
	GetConfig() (KubeVirtConfig, error)
}

// KubeVirtClusterConfigProvider is responsible for providing the current KubeVirt cluster config
type KubeVirtClusterConfigProvider struct {
	config.Provider
}

// NewKubeVirtConfigProvider creates new KubeVirt config provider that will ensure that the provided config is up to date
func NewKubeVirtConfigProvider(stopCh chan struct{}, clientset kubernetes.Interface, kubevirtNamespace string) KubeVirtClusterConfigProvider {
	return KubeVirtClusterConfigProvider{
		Provider: config.NewConfigProvider(stopCh, clientset, kubevirtNamespace, configMapName),
	}
}

// GetConfig provides the most current KubeVirt config
func (cp *KubeVirtClusterConfigProvider) GetConfig() (KubeVirtConfig, error) {
	bareConfig, err := cp.GetBareConfig(configMapName)
	return NewKubeVirtConfigFrom(bareConfig), err
}
