package framework

import (
	"time"

	v1 "kubevirt.io/client-go/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitForVMToExist blocks until VM is created
func (f *Framework) WaitForVMToExist(vmName string) (*v1.VirtualMachine, error) {
	var vm *v1.VirtualMachine
	pollErr := wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
		var err error
		vm, err = f.KubeVirtClient.VirtualMachine(f.Namespace.Name).Get(vmName, &metav1.GetOptions{})
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
