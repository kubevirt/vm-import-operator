package matchers

import (
	"sync/atomic"
	"time"

	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "kubevirt.io/client-go/api/v1"
)

type beRunningMatcher struct {
	pollingMatcher
	lastVirtualMachineInstance atomic.Value
}

// BeRunning creates the matcher
func BeRunning(testFramework *framework.Framework) types.GomegaMatcher {
	matcher := beRunningMatcher{}
	matcher.timeout = 5 * time.Minute
	matcher.testFramework = testFramework
	return &matcher
}

// Match checks whether given VM instance is running
func (matcher *beRunningMatcher) Match(actual interface{}) (bool, error) {
	vm := actual.(v1.VirtualMachine)
	pollErr := wait.PollImmediate(5*time.Second, matcher.timeout, func() (bool, error) {
		vmi, err := matcher.testFramework.KubeVirtClient.VirtualMachineInstance(vm.Namespace).Get(vm.Name, &metav1.GetOptions{})
		matcher.lastVirtualMachineInstance.Store(vmi)
		if err != nil {
			return false, err
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
