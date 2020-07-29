package utils

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

// VirtualMachineImportCr creates VM import CR with default name
func VirtualMachineImportCr(vmID string, namespace string, ovirtSecretName string, prefix string, startVM bool) v2vv1.VirtualMachineImport {
	return VirtualMachineImportCrWithName(vmID, namespace, ovirtSecretName, prefix, startVM, "target-vm")
}

// VirtualMachineImportCrWithName creates VM import CR with given name
func VirtualMachineImportCrWithName(vmID string, namespace string, ovirtSecretName string, prefix string, startVM bool, targetVMName string) v2vv1.VirtualMachineImport {
	return v2vv1.VirtualMachineImport{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
			Namespace:    namespace,
		},
		Spec: v2vv1.VirtualMachineImportSpec{
			StartVM: &startVM,
			ProviderCredentialsSecret: v2vv1.ObjectIdentifier{
				Name:      ovirtSecretName,
				Namespace: &namespace,
			},
			Source: v2vv1.VirtualMachineImportSourceSpec{
				Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{
					VM: v2vv1.VirtualMachineImportOvirtSourceVMSpec{ID: &vmID},
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
