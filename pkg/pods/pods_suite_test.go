package pods

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPodsManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodsManager Suite")
}
