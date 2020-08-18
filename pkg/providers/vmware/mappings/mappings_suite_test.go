package mappings_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMappings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mappings Suite")
}
