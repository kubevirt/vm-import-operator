package admission

import (
	"k8s.io/api/admission/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validating VirtualMachineImport Admitter", func() {
	vmImportAdmitter := &VirtualMachineImportAdmitter{}

	It("should accept VirtualMachineInstance spec", func() {

		ar := &v1beta1.AdmissionReview{
			Request: &v1beta1.AdmissionRequest{},
		}

		resp := vmImportAdmitter.Admit(ar)
		Expect(resp.Allowed).To(BeTrue())
	})
})
