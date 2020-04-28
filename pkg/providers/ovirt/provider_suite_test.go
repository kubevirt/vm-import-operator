package ovirtprovider

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOvirtProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ovirt provider Suite")
}
