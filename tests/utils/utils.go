package utils

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

// VirtualMachineImportCr creates VM import CR
func VirtualMachineImportCr(vmID string, namespace string, ovirtSecretName string, prefix string, startVm bool) v2vv1alpha1.VirtualMachineImport {
	targetVMName := "target-vm"
	return v2vv1alpha1.VirtualMachineImport{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
			Namespace:    namespace,
		},
		Spec: v2vv1alpha1.VirtualMachineImportSpec{
			StartVM: &startVm,
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

// FindTablet finds tablet device in given slice; nil is returned when tablet can't be found
func FindTablet(inputDevices []v1.Input) *v1.Input {
	var tablet *v1.Input
	for _, input := range inputDevices {
		if input.Type == "tablet" {
			tablet = &input
			break
		}
	}
	return tablet
}
