package framework

import (
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnsureVMImportDoesNotExist blocks until VM import with given name does not exist in the cluster
func (f *Framework) EnsureVMImportDoesNotExist(vmiName string) error {
	return wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
		_, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(f.Namespace.Name).Get(vmiName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

// WaitForVMImportConditionInStatus blocks until VM import with given name has given status condition with given status
func (f *Framework) WaitForVMImportConditionInStatus(pollInterval time.Duration, timeout time.Duration, vmiName string, conditionType v2vv1alpha1.VirtualMachineImportConditionType, status corev1.ConditionStatus, reason string, namespace string) error {
	pollErr := wait.PollImmediate(pollInterval, timeout, func() (bool, error) {
		retrieved, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(vmiName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		condition := conditions.FindConditionOfType(retrieved.Status.Conditions, conditionType)
		if condition == nil {
			return false, nil
		}
		if condition.Status != status {
			return false, nil
		}
		condReason := reason
		if condReason != "" {
			if *condition.Reason != condReason {
				return false, nil
			}
		}
		return true, nil
	})
	return pollErr
}

// WaitForVMToBeProcessing blocks until VM import with given name is in Processing state
func (f *Framework) WaitForVMToBeProcessing(vmiName string) error {
	return f.WaitForVMImportConditionInStatus(2*time.Second, time.Minute, vmiName, v2vv1alpha1.Processing, corev1.ConditionTrue, "", f.Namespace.Name)
}
