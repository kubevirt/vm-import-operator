package matchers

import (
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

type beSuccessfulMatcher struct {
	pollingMatcher
}

// BeSuccessful creates the matcher
func BeSuccessful(testFramework *framework.Framework) types.GomegaMatcher {
	matcher := beSuccessfulMatcher{}
	matcher.timeout = 5 * time.Minute
	matcher.testFramework = testFramework
	return &matcher
}

// Match polls cluster until the virtual machine import is marked as successful
func (matcher *beSuccessfulMatcher) Match(actual interface{}) (bool, error) {
	vmBluePrint := actual.(*v2vv1alpha1.VirtualMachineImport)
	pollErr := wait.PollImmediate(5*time.Second, matcher.timeout, func() (bool, error) {
		retrieved, err := matcher.testFramework.VMImportClient.V2vV1alpha1().VirtualMachineImports(vmBluePrint.Namespace).Get(vmBluePrint.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		succeededCondition := conditions.FindConditionOfType(retrieved.Status.Conditions, v2vv1alpha1.Succeeded)
		if succeededCondition == nil {
			return false, nil
		}
		if succeededCondition.Status != corev1.ConditionTrue {
			return false, nil
		}
		return true, nil
	})
	if pollErr != nil {
		return false, pollErr
	}
	return true, nil
}

// FailureMessage is a message shown for failure
func (matcher *beSuccessfulMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be a successful VirtualMachineImport")
}

// NegatedFailureMessage us  message shown for negated failure
func (matcher *beSuccessfulMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to be a successful VirtualMachineImport")
}
