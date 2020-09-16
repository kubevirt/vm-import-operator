package jobs

import (
"testing"

. "github.com/onsi/ginkgo"
. "github.com/onsi/gomega"
)

func TestJobsManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "JobsManager Suite")
}
