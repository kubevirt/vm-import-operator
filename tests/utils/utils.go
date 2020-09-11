package utils

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

// VirtualMachineImportCr creates VM import CR with default name
func VirtualMachineImportCr(provider string, vmID string, namespace string, secretName string, prefix string, startVM bool) v2vv1.VirtualMachineImport {
	return VirtualMachineImportCrWithName(provider, vmID, namespace, secretName, prefix, startVM, "target-vm")
}

// VirtualMachineImportCrWithName creates VM import CR with given name
func VirtualMachineImportCrWithName(provider string, vmID string, namespace string, secretName string, prefix string, startVM bool, targetVMName string) v2vv1.VirtualMachineImport {
	vm := v2vv1.VirtualMachineImport{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
			Namespace:    namespace,
		},
		Spec: v2vv1.VirtualMachineImportSpec{
			StartVM: &startVM,
			ProviderCredentialsSecret: v2vv1.ObjectIdentifier{
				Name:      secretName,
				Namespace: &namespace,
			},
			TargetVMName: &targetVMName,
		},
	}
	switch provider {
	case framework.ProviderOvirt:
		vm.Spec.Source = v2vv1.VirtualMachineImportSourceSpec{
			Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{
				VM: v2vv1.VirtualMachineImportOvirtSourceVMSpec{ID: &vmID},
			},
		}
	case framework.ProviderVmware:
		vm.Spec.Source = v2vv1.VirtualMachineImportSourceSpec{
			Vmware: &v2vv1.VirtualMachineImportVmwareSourceSpec{
				VM: v2vv1.VirtualMachineImportVmwareSourceVMSpec{ID: &vmID},
			},
		}
	}
	return vm
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
