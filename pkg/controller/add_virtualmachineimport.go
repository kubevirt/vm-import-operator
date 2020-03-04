package controller

import (
	"github.com/kubevirt/vm-import-operator/pkg/controller/virtualmachineimport"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, virtualmachineimport.Add)
}
