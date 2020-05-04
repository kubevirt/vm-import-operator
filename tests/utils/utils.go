package utils

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualMachineImportCr creates VM import CR
func VirtualMachineImportCr(vmID string, namespace string, ovirtSecretName string, prefix string) v2vv1alpha1.VirtualMachineImport {
	targetVMName := "target-vm"
	return v2vv1alpha1.VirtualMachineImport{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
			Namespace:    namespace,
		},
		Spec: v2vv1alpha1.VirtualMachineImportSpec{
			ProviderCredentialsSecret: v2vv1alpha1.ObjectIdentifier{
				Name:      ovirtSecretName,
				Namespace: &namespace,
			},
			Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
				Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{
					VM: v2vv1alpha1.VirtualMachineImportOvirtSourceVMSpec{ID: &vmID},
				},
			},
			TargetVMName: &targetVMName,
		},
	}
}
