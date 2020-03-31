package mapper

import (
	"strconv"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	ovirtsdk "github.com/ovirt/go-ovirt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	cdiAPIVersion = "cdi.kubevirt.io/v1alpha1"
)

var biosMapping = map[string]*kubevirtv1.Bootloader{
	"q35_sea_bios":    &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}},
	"q35_secure_boot": &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}},
	"q35_ovmf":        &kubevirtv1.Bootloader{EFI: &kubevirtv1.EFI{}},
}

var archMapping = map[string]string{
	"x86_64": "q35",
}

// OvirtMapper is struct that holds attributes needed to map oVirt VM to kubevirt VM
type OvirtMapper struct {
	vm        *ovirtsdk.Vm
	mappings  *v2vv1alpha1.OvirtMappings
	creds     provider.DataVolumeCredentials
	namespace string
}

// NewOvirtMapper create ovirt mapper object
func NewOvirtMapper(vm *ovirtsdk.Vm, mappings *v2vv1alpha1.OvirtMappings, creds provider.DataVolumeCredentials, namespace string) *OvirtMapper {
	return &OvirtMapper{
		vm:        vm,
		mappings:  mappings,
		creds:     creds,
		namespace: namespace,
	}
}

// MapVM map oVirt API VM definition to kubevirt VM definition
func (o *OvirtMapper) MapVM(targetVMName *string) *kubevirtv1.VirtualMachine {
	vmSpec := kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.namespace,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{},
				},
			},
		},
	}

	// Map name
	vmName, shouldGenerate := o.resolveVMName(targetVMName, o.vm)
	if shouldGenerate {
		vmSpec.ObjectMeta.GenerateName = "ovirt-"
	} else {
		vmSpec.ObjectMeta.Name = vmName
	}

	// Map CPU
	vmSpec.Spec.Template.Spec.Domain.CPU = o.mapCPU()

	// Map bios
	vmSpec.Spec.Template.Spec.Domain.Firmware = o.mapFirmware()

	// Map machine type
	vmSpec.Spec.Template.Spec.Domain.Machine = *o.mapArchitecture()

	return &vmSpec
}

// MapDisks map the oVirt VM disks to the map of CDI DataVolumes specification, where
// map id is the id of the oVirt disk
func (o *OvirtMapper) MapDisks() map[string]cdiv1.DataVolume {
	diskAttachments, _ := o.vm.DiskAttachments()
	dvs := make(map[string]cdiv1.DataVolume, len(diskAttachments.Slice()))

	for _, diskAttachment := range diskAttachments.Slice() {
		attachID, _ := diskAttachment.Id()
		disk, _ := diskAttachment.Disk()
		diskID, _ := disk.Id()
		sdClass := o.getStorageClassForDisk(disk, o.mappings)
		diskSize, _ := disk.ProvisionedSize()
		quantity, _ := resource.ParseQuantity(strconv.FormatInt(diskSize, 10))

		dvs[attachID] = cdiv1.DataVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: cdiAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      attachID,
				Namespace: o.namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: cdiv1.DataVolumeSource{
					Imageio: &cdiv1.DataVolumeSourceImageIO{
						URL:           o.creds.URL,
						DiskID:        diskID,
						SecretRef:     o.creds.SecretName,
						CertConfigMap: o.creds.ConfigMapName,
					},
				},
				PVC: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: quantity,
						},
					},
				},
			},
		}
		if sdClass != nil {
			dvs[attachID].Spec.PVC.StorageClassName = sdClass
		}
	}
	return dvs
}

func (o *OvirtMapper) resolveVMName(targetVMName *string, vm *ovirtsdk.Vm) (string, bool) {
	if targetVMName != nil {
		return *targetVMName, false
	}

	name, ok := vm.Name()
	if !ok {
		return "", true
	}

	name, err := utils.NormalizeName(name)
	if err != nil {
		// TODO: should name validation be included in condition ?
		return "", true
	}

	return name, false
}

func (o *OvirtMapper) getStorageClassForDisk(disk *ovirtsdk.Disk, mappings *v2vv1alpha1.OvirtMappings) *string {
	sds, _ := disk.StorageDomains()
	sd := sds.Slice()[0]
	for _, mapping := range *mappings.StorageMappings {
		if mapping.Source.Name != nil {
			if name, _ := sd.Name(); name == *mapping.Source.Name {
				return &mapping.Target.Name
			}
		}

		if mapping.Source.ID != nil {
			if id, _ := sd.Id(); id == *mapping.Source.ID {
				return &mapping.Target.Name
			}
		}
	}

	// Use default storage class:
	return nil
}

func (o *OvirtMapper) mapArchitecture() *kubevirtv1.Machine {
	arch, _ := o.vm.MustCpu().Architecture()
	return &kubevirtv1.Machine{
		Type: archMapping[string(arch)],
	}
}

func (o *OvirtMapper) mapFirmware() *kubevirtv1.Firmware {
	bios, _ := o.vm.Bios()
	biosType, _ := bios.Type()
	bootloader, ok := biosMapping[string(biosType)]
	if ok {
		bootloader = &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}}
	}
	return &kubevirtv1.Firmware{
		Bootloader: bootloader,
	}
}

func (o *OvirtMapper) mapCPU() *kubevirtv1.CPU {
	cpu := kubevirtv1.CPU{}
	if cpuDef, available := o.vm.Cpu(); available {
		if topology, available := cpuDef.Topology(); available {
			if cores, available := topology.Cores(); available {
				cpu.Cores = uint32(cores)
			}
			if sockets, available := topology.Sockets(); available {
				cpu.Sockets = uint32(sockets)
			}
			if threads, available := topology.Threads(); available {
				cpu.Threads = uint32(threads)
			}
		}
	}

	return &cpu
}
