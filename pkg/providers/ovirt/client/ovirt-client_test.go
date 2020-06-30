package ovirtclient

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("oVirt client", func() {

	client := richOvirtClient{
		// nil connection will cause panic
		connection: nil,
	}

	It("should recover from VM retrieval panic", func() {
		vmID := "any"
		vm, err := client.GetVM(&vmID, nil, nil, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("panicked"))
		Expect(vm).To(BeNil())
	})
	It("should recover from VM starting panic", func() {
		err := client.StartVM("any")

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("panicked"))
	})
	It("should recover from VM stopping panic", func() {
		err := client.StopVM("any")

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("panicked"))
	})
})
