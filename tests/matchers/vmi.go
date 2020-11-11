package matchers

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"sync/atomic"
	"time"

	"github.com/onsi/ginkgo"

	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "kubevirt.io/client-go/api/v1"
)

type beRunningMatcher struct {
	pollingMatcher
	lastVirtualMachineInstance atomic.Value
}

// BeRunning creates the matcher
func BeRunning(testFramework *framework.Framework) *beRunningMatcher {
	matcher := beRunningMatcher{}
	matcher.timeout = 15 * time.Minute
	matcher.testFramework = testFramework
	return &matcher
}

// Timeout sets timeout on the matcher
func (matcher *beRunningMatcher) Timeout(timeout time.Duration) *beRunningMatcher {
	matcher.timeout = timeout
	return matcher
}

// Match checks whether given VM instance is running
func (matcher *beRunningMatcher) Match(actual interface{}) (bool, error) {
	vm := actual.(v1.VirtualMachine)
	pollErr := wait.PollImmediate(5*time.Second, matcher.timeout, func() (bool, error) {
		vmi := &v1.VirtualMachineInstance{}
		vmiNamespacedName := types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}

		err := matcher.testFramework.Client.Get(context.TODO(), vmiNamespacedName, vmi)
		matcher.lastVirtualMachineInstance.Store(vmi)
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: VM instance polling error: %v\n", err)
			return false, nil
		}
		if vmi.Status.Phase == v1.Running {
			return true, nil
		}
		return false, nil
	})
	if pollErr != nil {
		return false, nil
	}
	return true, nil
}

// FailureMessage is a message shown for failure
func (matcher *beRunningMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(matcher.lastVirtualMachineInstance.Load(), "to be a running VirtualMachineInstance")
}

// NegatedFailureMessage us  message shown for negated failure
func (matcher *beRunningMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(matcher.lastVirtualMachineInstance.Load(), "not to be a running VirtualMachineInstance")
}
