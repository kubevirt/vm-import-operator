package controller

import (
	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, ctrlConfig.ControllerConfigProvider) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, ctrlConfigProvider ctrlConfig.ControllerConfigProvider) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, ctrlConfigProvider); err != nil {
			return err
		}
	}
	return nil
}
