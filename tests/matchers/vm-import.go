package matchers

import (
	"fmt"
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

type hasConditionInStatus struct {
	pollingMatcher
	conditionType v2vv1alpha1.VirtualMachineImportConditionType
	status        corev1.ConditionStatus
}

// HaveMappingRulesVerificationFailure creates the matcher checking whether Virtual Machine Import has failed mapping rules verification
func HaveMappingRulesVerificationFailure(testFramework *framework.Framework) types.GomegaMatcher {
	matcher := hasConditionInStatus{}
	matcher.timeout = 1 * time.Minute
	matcher.pollInterval = 1 * time.Second
	matcher.testFramework = testFramework

	matcher.conditionType = v2vv1alpha1.MappingRulesVerified
	matcher.status = corev1.ConditionFalse
	return &matcher
}

// BeProcessing creates the matcher checking whether Virtual Machine Import is currently processing
func BeProcessing(testFramework *framework.Framework) types.GomegaMatcher {
	matcher := hasConditionInStatus{}
	matcher.timeout = 5 * time.Minute
	matcher.pollInterval = 2 * time.Second
	matcher.testFramework = testFramework

	matcher.conditionType = v2vv1alpha1.Processing
	matcher.status = corev1.ConditionTrue
	return &matcher
}

// BeSuccessful creates the matcher checking whether Virtual Machine Import is successful
func BeSuccessful(testFramework *framework.Framework) types.GomegaMatcher {
	matcher := hasConditionInStatus{}
	matcher.timeout = 5 * time.Minute
	matcher.pollInterval = 5 * time.Second
	matcher.testFramework = testFramework

	matcher.conditionType = v2vv1alpha1.Succeeded
	matcher.status = corev1.ConditionTrue
	return &matcher
}

// Match polls cluster until the virtual machine import is marked as expected
func (matcher *hasConditionInStatus) Match(actual interface{}) (bool, error) {
	vmBluePrint := actual.(*v2vv1alpha1.VirtualMachineImport)
	pollErr := matcher.testFramework.WaitForVMImportConditionInStatus(
		matcher.pollInterval,
		matcher.timeout,
		vmBluePrint.Name,
		matcher.conditionType,
		matcher.status,
	)
	if pollErr != nil {
		return false, pollErr
	}
	return true, nil
}

// FailureMessage is a message shown for failure
func (matcher *hasConditionInStatus) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be a VirtualMachineImport with condition in status", matcher.expectedValue())
}

// NegatedFailureMessage us  message shown for negated failure
func (matcher *hasConditionInStatus) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to be a VirtualMachineImport with condition in status", matcher.expectedValue())
}

func (matcher *hasConditionInStatus) expectedValue() string {
	return fmt.Sprintf("%v:%v", matcher.conditionType, matcher.status)
}
