package controller

import (
	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, kvConfig.KubeVirtConfigProvider) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, configProvider kvConfig.KubeVirtConfigProvider) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, configProvider); err != nil {
			return err
		}
	}
	return nil
}
