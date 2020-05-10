package os_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOsMapperProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OS Mapper Provider Suite")
}
