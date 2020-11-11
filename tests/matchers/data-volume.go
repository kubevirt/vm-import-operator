package matchers

import (
	"context"
	"fmt"

	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type haveStorageClassMatcher struct {
	storageClass  *string
	testFramework *framework.Framework
}

// HaveDefaultStorageClass whether data volume of a given name has default storage class (nil)
func HaveDefaultStorageClass(f *framework.Framework) types.GomegaMatcher {
	return &haveStorageClassMatcher{testFramework: f}
}

// HaveStorageClass creates matcher checking whether data volume of a given name has expected storage class
func HaveStorageClass(sc string, f *framework.Framework) types.GomegaMatcher {
	return &haveStorageClassMatcher{storageClass: &sc, testFramework: f}
}

// HaveStorageClassReference creates matcher checking whether data volume of a given name has expected storage class
func HaveStorageClassReference(sc *string, f *framework.Framework) types.GomegaMatcher {
	return &haveStorageClassMatcher{storageClass: sc, testFramework: f}
}

// Match checks whether data volume of a given name has expected storage class
func (matcher *haveStorageClassMatcher) Match(dvName interface{}) (bool, error) {
	f := matcher.testFramework
	dv, err := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dvName.(string), v1.GetOptions{})
	if err != nil {
		return false, err
	}
	actual := dv.Spec.PVC.StorageClassName
	expected := matcher.storageClass
	if expected == nil && actual == nil {
		return true, nil
	}
	if expected != nil && actual != nil && *expected == *actual {
		return true, nil
	}
	return false, nil
}

// FailureMessage is a message shown for failure
func (matcher *haveStorageClassMatcher) FailureMessage(actual interface{}) (message string) {
	dvName := actual.(string)
	if matcher.storageClass != nil {
		return fmt.Sprintf("Expected\nData volume %s\nto have storage class\n%s", dvName, *matcher.storageClass)
	} else {
		return fmt.Sprintf("Expected\nData volume %s\nto have no storage class", dvName)
	}
}

// NegatedFailureMessage us  message shown for negated failure
func (matcher *haveStorageClassMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	dvName := actual.(string)
	if matcher.storageClass != nil {
		return fmt.Sprintf("Expected\nData volume %s\nnot to have storage class\n%s", dvName, *matcher.storageClass)
	} else {
		return fmt.Sprintf("Expected\nData volume %s\nto have storage class", dvName)
	}
}
