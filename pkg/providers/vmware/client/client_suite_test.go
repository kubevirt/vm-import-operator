package client_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestVmwareRichClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VmwareRichClient Suite")
}
