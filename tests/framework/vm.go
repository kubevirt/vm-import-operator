package framework

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	"time"

	v1 "kubevirt.io/client-go/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitForVMToExist blocks until VM is created
func (f *Framework) WaitForVMToExist(vmName string) (*v1.VirtualMachine, error) {
	var vm *v1.VirtualMachine
	pollErr := wait.PollImmediate(2*time.Second, 2*time.Minute, func() (bool, error) {
		var err error
		vm = &v1.VirtualMachine{}
		vmNamespacedName := types.NamespacedName{
			Namespace: f.Namespace.Name,
			Name:      vmName,
		}

		err = f.Client.Get(context.TODO(), vmNamespacedName, vm)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return vm, pollErr
}
