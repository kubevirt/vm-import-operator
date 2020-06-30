package ovirtclient_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOvirtClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "oVirt client Suite")
}
