package guestconversion

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGuestConversion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GuestConversion Suite")
}
