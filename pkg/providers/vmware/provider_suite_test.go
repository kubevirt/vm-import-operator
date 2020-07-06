package vmware

import (
	"testing"
	
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestVmwareProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vmware provider suite")
}