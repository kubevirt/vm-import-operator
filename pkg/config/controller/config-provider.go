package controller

import (
	"github.com/kubevirt/vm-import-operator/pkg/config"
	"k8s.io/client-go/kubernetes"
)

const (
	configMapName = "vm-import-controller-config"
)

// ControllerConfigProvider defines controller config access operations
type ControllerConfigProvider interface {
	GetConfig() (ControllerConfig, error)
}

// ConfigMapControllerConfigProvider is responsible for providing the current controller config
type ConfigMapControllerConfigProvider struct {
	config.Provider
}

// NewControllerConfigProvider creates new controller config provider that will ensure that the provided config is up to date
func NewControllerConfigProvider(stopCh chan struct{}, clientset kubernetes.Interface, controllerNamespace string) ConfigMapControllerConfigProvider {
	return ConfigMapControllerConfigProvider{
		Provider: config.NewConfigProvider(stopCh, clientset, controllerNamespace, configMapName),
	}
}

// GetConfig provides the most current controller config
func (cp *ConfigMapControllerConfigProvider) GetConfig() (ControllerConfig, error) {
	bareConfig, err := cp.GetBareConfig(configMapName)
	return NewControllerConfigFrom(bareConfig), err
}
