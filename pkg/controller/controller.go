package controller

import (
	"github.com/kubevirt/vm-import-operator/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, config.KubeVirtConfigProvider) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, configProvider config.KubeVirtConfigProvider) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, configProvider); err != nil {
			return err
		}
	}
	return nil
}
