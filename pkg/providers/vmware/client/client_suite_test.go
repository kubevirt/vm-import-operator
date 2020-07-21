package client_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestVmwareRichClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VmwareRichClient Suite")
}
