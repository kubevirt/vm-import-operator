package controller

import (
	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, kvConfig.KubeVirtConfigProvider, ctrlConfig.ControllerConfigProvider) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, kvConfigProvider kvConfig.KubeVirtConfigProvider, ctrlConfigProvider ctrlConfig.ControllerConfigProvider) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, kvConfigProvider, ctrlConfigProvider); err != nil {
			return err
		}
	}
	return nil
}
