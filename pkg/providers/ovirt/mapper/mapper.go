package mapper

import (
	"fmt"
	"strconv"
	"strings"

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
	cdiAPIVersion           = "cdi.kubevirt.io/v1alpha1"
	customPropertyHugePages = "hugepages"
	// LabelOrigin define name of the label which holds value of origin attribute of oVirt VM
	LabelOrigin = "origin"
	// LabelInstanceType define name of the label which holds value of instance type attribute of oVirt VM
	LabelInstanceType = "instanceType"
	// LabelTag define name of the label which holds tags of oVirt VM
	LabelTag = "tags"
	// AnnotationComment define name of the annotation which holds comment of oVirt VM
	AnnotationComment = "comment"
	// AnnotationSso define name of the annotation which holds sso method of oVirt VM
	AnnotationSso = "sso"
)

// BiosTypeMapping defines mapping of BIOS types between oVirt and kubevirt domains
var BiosTypeMapping = map[string]*kubevirtv1.Bootloader{
	"q35_sea_bios":    &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}},
	"q35_secure_boot": &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}},
	"q35_ovmf":        &kubevirtv1.Bootloader{EFI: &kubevirtv1.EFI{}},
}

var archMapping = map[string]string{
	"x86_64": "q35",
	"ppc64":  "pseries",
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

	// Map hostname
	if fqdn, ok := o.vm.Fqdn(); ok {
		vmSpec.Spec.Template.Spec.Hostname = fqdn
	}

	// Map CPU
	vmSpec.Spec.Template.Spec.Domain.CPU = o.mapCPU()

	// Map bios
	vmSpec.Spec.Template.Spec.Domain.Firmware = o.mapFirmware()

	// Map machine type
	vmSpec.Spec.Template.Spec.Domain.Machine = *o.mapArchitecture()

	// High Availability is mapped to running field of VM spec
	vmSpec.Spec.Running = o.mapHighAvailability()

	// Memory set hugepages and guest memory
	vmSpec.Spec.Template.Spec.Domain.Memory = o.mapMemory()

	// Memory policy set the memory limit
	vmSpec.Spec.Template.Spec.Domain.Resources = o.mapResourceRequirements()

	// Devices
	vmSpec.Spec.Template.Spec.Domain.Devices = *o.mapGraphicalConsoles()

	// Map labels like origin, instance_type
	vmSpec.ObjectMeta.Labels = o.mapLabels()

	// Map annotations like sso
	vmSpec.ObjectMeta.Annotations = o.mapAnnotations()

	// Map placement policy
	vmSpec.Spec.Template.Spec.EvictionStrategy = o.mapPlacementPolicy()

	// Map timezone
	vmSpec.Spec.Template.Spec.Domain.Clock = o.mapTimeZone()

	return &vmSpec
}

// MapDisks map the oVirt VM disks to the map of CDI DataVolumes specification, where
// map id is the id of the oVirt disk
func (o *OvirtMapper) MapDisks() map[string]cdiv1.DataVolume {
	// TODO: stateless, boot_devices, floppy/cdrom
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
	sd, _ := disk.StorageDomain()
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
	machine := &kubevirtv1.Machine{
		Type: archMapping[string(arch)],
	}

	// Override machine type in case custom emulated machine is specified
	if custom, ok := o.vm.CustomEmulatedMachine(); ok {
		machine.Type = custom
	}

	return machine
}

func (o *OvirtMapper) mapFirmware() *kubevirtv1.Firmware {
	firmware := &kubevirtv1.Firmware{}

	// Map bootloader
	bios, _ := o.vm.Bios()
	biosType, _ := bios.Type()
	bootloader, ok := BiosTypeMapping[string(biosType)]
	if ok {
		bootloader = &kubevirtv1.Bootloader{BIOS: &kubevirtv1.BIOS{}}
	}
	firmware.Bootloader = bootloader

	// Map serial number
	serial := o.mapSerialNumber()
	if serial != nil {
		firmware.Serial = *serial
	}

	return firmware
}

func (o *OvirtMapper) mapSerialNumber() *string {
	if serialNumber, ok := o.vm.SerialNumber(); ok {
		var serial string
		policy, _ := serialNumber.Policy()
		if policy == ovirtsdk.SERIALNUMBERPOLICY_VM {
			serial, _ = o.vm.Id()
		} else if policy == ovirtsdk.SERIALNUMBERPOLICY_CUSTOM {
			serial, _ = serialNumber.Value()
		} else {
			return nil
		}
		return &serial
	}

	return nil
}

func (o *OvirtMapper) mapCPU() *kubevirtv1.CPU {
	cpu := kubevirtv1.CPU{}

	// CPU topology
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

	// Custom cpu model
	if customCPU, available := o.vm.CustomCpuModel(); available {
		cpu.Model = customCPU
	}

	return &cpu
}

func (o *OvirtMapper) mapHighAvailability() *bool {
	enabled := false
	ha, _ := o.vm.HighAvailability()
	if enabled, _ := ha.Enabled(); enabled {
		return &enabled
	}

	return &enabled
}

func (o *OvirtMapper) mapMemory() *kubevirtv1.Memory {
	// Guest memory
	ovirtVMMemory, _ := o.vm.Memory()
	guestMemory, _ := resource.ParseQuantity(strconv.FormatInt(ovirtVMMemory, 10))
	memory := &kubevirtv1.Memory{}
	memory.Guest = &guestMemory

	// HugePages
	hp := o.mapHugePages()
	if hp != nil {
		memory.Hugepages = hp
	}

	return memory
}

func (o *OvirtMapper) mapHugePages() *kubevirtv1.Hugepages {
	var hugePageSize *string
	if customProperties, available := o.vm.CustomProperties(); available {
		for _, cp := range customProperties.Slice() {
			if cpName, _ := cp.Name(); cpName == customPropertyHugePages {
				cmValue, _ := cp.Value()
				hugePageSize = &cmValue
				break
			}
		}
	}
	if hugePageSize != nil {
		return &kubevirtv1.Hugepages{
			// HugePage size in oVirt custom_property is in MiB, so we add Mi suffix here.
			PageSize: fmt.Sprintf("%sMi", *hugePageSize),
		}
	}

	return nil
}

func (o *OvirtMapper) mapResourceRequirements() kubevirtv1.ResourceRequirements {
	reqs := kubevirtv1.ResourceRequirements{}
	memoryPolicy, _ := o.vm.MemoryPolicy()
	maxMemory, _ := memoryPolicy.Max()
	maxMemoryQuantity, _ := resource.ParseQuantity(strconv.FormatInt(maxMemory, 10))
	reqs.Limits = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceMemory: maxMemoryQuantity,
	}

	return reqs
}

// Graphical console is attached only in case the VM is not in headless mode
func (o *OvirtMapper) mapGraphicalConsoles() *kubevirtv1.Devices {
	// GraphicsConsole
	vncEnabled := true
	if gc, _ := o.vm.GraphicsConsoles(); len(gc.Slice()) == 0 {
		vncEnabled = false
	}
	devices := &kubevirtv1.Devices{}
	devices.AutoattachGraphicsDevice = &vncEnabled

	// SerialConsole
	if console, ok := o.vm.Console(); ok {
		consoleEnabled, _ := console.Enabled()
		devices.AutoattachSerialConsole = &consoleEnabled
	}

	return devices
}

func (o *OvirtMapper) mapLabels() map[string]string {
	labels := map[string]string{}

	// Origin
	labels[LabelOrigin], _ = o.vm.Origin()

	// Instance type
	if instanceType, ok := o.vm.InstanceType(); ok {
		labels[LabelInstanceType], _ = instanceType.Name()
	}

	// Tags
	if tags, ok := o.vm.Tags(); ok {
		var tagList []string
		for _, tag := range tags.Slice() {
			tagName, _ := tag.Name()
			tagList = append(tagList, tagName)
		}
		labels[LabelTag] = strings.Join(tagList, ",")
	}

	return labels
}

func (o *OvirtMapper) mapAnnotations() map[string]string {
	annotations := map[string]string{}

	// Comment
	if comment, ok := o.vm.Comment(); ok {
		annotations[AnnotationComment] = comment
	}

	// SSO
	if sso, ok := o.vm.Sso(); ok {
		if methods, _ := sso.Methods(); len(methods.Slice()) > 0 {
			annotations[AnnotationSso] = string(ovirtsdk.SSOMETHOD_GUEST_AGENT)
		} else {
			annotations[AnnotationSso] = "disabled"
		}
	}

	return annotations
}

func (o *OvirtMapper) mapPlacementPolicy() *kubevirtv1.EvictionStrategy {
	placementPolicy, _ := o.vm.PlacementPolicy()
	affinity, _ := placementPolicy.Affinity()
	if affinity == ovirtsdk.VMAFFINITY_MIGRATABLE {
		strategy := kubevirtv1.EvictionStrategyLiveMigrate
		return &strategy
	}

	return nil
}

func (o *OvirtMapper) mapTimeZone() *kubevirtv1.Clock {
	clock := &kubevirtv1.Clock{}
	if timezone, ok := o.vm.TimeZone(); ok {
		timezoneName, _ := timezone.Name()
		clockOffsetTimezone := kubevirtv1.ClockOffsetTimezone(timezoneName)
		clock.Timezone = &clockOffsetTimezone
	}
	return clock
}
