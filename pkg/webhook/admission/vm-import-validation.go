package admission

import (
	"k8s.io/api/admission/v1beta1"
)

// VirtualMachineImportAdmitter validates VirtualMachineImport object
type VirtualMachineImportAdmitter struct {
}

// Admit validates whether VM described in VirtualMachineImport can be imported
func (admitter *VirtualMachineImportAdmitter) Admit(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}
