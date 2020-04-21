package matchers

import (
	"time"

	"github.com/kubevirt/vm-import-operator/tests/framework"
)

type pollingMatcher struct {
	timeout       time.Duration
	testFramework *framework.Framework
}
