package mapper

import (
	"crypto/sha1"
	"fmt"
	"strings"

	oos "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/os"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	outils "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/utils"
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
	networkTypePod          = "pod"
	networkTypeMultus       = "multus"
	dataVolumeKind          = "DataVolume"
	vmNamePrefix            = "ovirt-"
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
	// DefaultStorageClassTargetName define the storage target name value that forces using default storage class
	DefaultStorageClassTargetName = ""
)

var (
	_true bool = true
	// DefaultVolumeMode is default volume mode passed to PVC of datavolume we map.
	DefaultVolumeMode = corev1.PersistentVolumeFilesystem
)

// DiskInterfaceModelMapping defines mapping of disk interface models between oVirt and kubevirt domains
var DiskInterfaceModelMapping = map[string]string{"sata": "sata", "virtio_scsi": "scsi", "virtio": "virtio"}

// BiosTypeMapping defines mapping of BIOS types between oVirt and kubevirt domains
var BiosTypeMapping = map[string]*kubevirtv1.Bootloader{
	"q35_sea_bios":    {BIOS: &kubevirtv1.BIOS{}},
	"q35_secure_boot": {BIOS: &kubevirtv1.BIOS{}},
	"q35_ovmf":        {EFI: &kubevirtv1.EFI{}},
	"i440fx_sea_bios": {BIOS: &kubevirtv1.BIOS{}},
}

var archMapping = map[string]string{
	"x86_64": "q35",
	"ppc64":  "pseries",
}

var log = logf.Log.WithName("mapper")

// DataVolumeCredentials defines the credentials required for creating a data volume
type DataVolumeCredentials struct {
	URL           string
	CACertificate string
	KeyAccess     string
	KeySecret     string
	ConfigMapName string
	SecretName    string
}

// OvirtMapper is struct that holds attributes needed to map oVirt VM to kubevirt VM
type OvirtMapper struct {
	vm        *ovirtsdk.Vm
	mappings  *v2vv1.OvirtMappings
	creds     DataVolumeCredentials
	namespace string
	osFinder  oos.OSFinder
}

// NewOvirtMapper create ovirt mapper object
func NewOvirtMapper(vm *ovirtsdk.Vm, mappings *v2vv1.OvirtMappings, creds DataVolumeCredentials, namespace string, osFinder oos.OSFinder) *OvirtMapper {
	return &OvirtMapper{
		vm:        vm,
		mappings:  mappings,
		creds:     creds,
		namespace: namespace,
		osFinder:  osFinder,
	}
}

// CreateEmptyVM creates empty virtual machine definition
func (o *OvirtMapper) CreateEmptyVM(vmName *string) *kubevirtv1.VirtualMachine {
	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": *vmName,
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"kubevirt.io/domain":  *vmName,
						"vm.kubevirt.io/name": *vmName,
					},
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{},
				},
			},
		},
	}
}

// MapVM map oVirt API VM definition to kubevirt VM definition
func (o *OvirtMapper) MapVM(targetVMName *string, vmSpec *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	var err error
	// Set Namespace
	vmSpec.ObjectMeta.Namespace = o.namespace

	// Map name
	if targetVMName == nil {
		vmSpec.ObjectMeta.GenerateName = vmNamePrefix
	} else {
		vmSpec.ObjectMeta.Name = *targetVMName
	}

	if vmSpec.Spec.Template == nil {
		vmSpec.Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{}
	}

	// Map hostname
	if fqdn, ok := o.vm.Fqdn(); ok {
		name, _ := utils.NormalizeLabel(fqdn)
		vmSpec.Spec.Template.Spec.Hostname = name
	}

	// Map CPU
	cpu := o.mapCPU()
	vmSpec.Spec.Template.Spec.Domain.CPU = cpu

	// Map bios
	vmSpec.Spec.Template.Spec.Domain.Firmware = o.mapFirmware()

	// Map features
	vmSpec.Spec.Template.Spec.Domain.Features = o.mapFeatures()

	// Map machine type
	vmSpec.Spec.Template.Spec.Domain.Machine = *o.mapArchitecture()

	// High Availability is mapped to running field of VM spec
	vmSpec.Spec.Running = o.mapHighAvailability()

	// Memory set hugepages and guest memory
	vmSpec.Spec.Template.Spec.Domain.Memory = o.mapMemory()

	// Memory policy set the memory limit
	if vmSpec.Spec.Template.Spec.Domain.Resources, err = o.mapResourceRequirements(cpu.DedicatedCPUPlacement); err != nil {
		return vmSpec, err
	}

	os, _ := o.osFinder.FindOperatingSystem(o.vm)

	// Devices
	vmSpec.Spec.Template.Spec.Domain.Devices = *o.mapGraphicalConsoles(os)

	// Map labels like origin, instance_type
	vmSpec.ObjectMeta.Labels = o.mapLabels(vmSpec.ObjectMeta.Labels)

	// Map annotations like sso
	vmSpec.ObjectMeta.Annotations = o.mapAnnotations()

	// Map placement policy
	vmSpec.Spec.Template.Spec.EvictionStrategy = o.mapPlacementPolicy()

	// Map timezone
	vmSpec.Spec.Template.Spec.Domain.Clock = o.mapTimeZone()

	// Map networks
	vmSpec.Spec.Template.Spec.Networks = o.mapNetworks()

	// Map network interfaces
	networkToType := o.mapNetworksToTypes(vmSpec.Spec.Template.Spec.Networks)
	vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces = o.mapNics(networkToType)

	return vmSpec, nil
}

// MapDisk map VM disk
func (o *OvirtMapper) MapDisk(vmSpec *kubevirtv1.VirtualMachine, dv cdiv1.DataVolume) {
	// Map volume
	name := fmt.Sprintf("dv-%v", dv.Name)
	name = utils.EnsureLabelValueLength(name)
	volume := kubevirtv1.Volume{
		Name: name,
		VolumeSource: kubevirtv1.VolumeSource{
			DataVolume: &kubevirtv1.DataVolumeSource{
				Name: dv.Name,
			},
		},
	}

	// Map disks
	diskAttachments, _ := o.vm.DiskAttachments()
	diskAttachment := getDiskAttachmentByID(dv.Name, diskAttachments, vmSpec.ObjectMeta.Name)
	iface, _ := diskAttachment.Interface()
	disk := kubevirtv1.Disk{
		Name: name,
		DiskDevice: kubevirtv1.DiskDevice{
			Disk: &kubevirtv1.DiskTarget{
				Bus: DiskInterfaceModelMapping[string(iface)],
			},
		},
	}
	if bootable, ok := diskAttachment.Bootable(); ok && bootable {
		bootOrder := uint(1)
		disk.BootOrder = &bootOrder
	}

	vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, volume)
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, disk)
}

// RunningState determines whether the created Kubevirt vmSpec should
// have a running state of true or false.
func (o *OvirtMapper) RunningState() bool {
	running := o.mapHighAvailability()
	if running != nil {
		return *running
	}
	return false
}

// If the mapping specifies the access mode return that, otherwise determine the access mode
// of the PVC based on the VM's disk read only attribute and based on the affinity settings of the VM.
func (o *OvirtMapper) getAccessMode(diskAttachment *ovirtsdk.DiskAttachment, mapping *v2vv1.StorageResourceMappingItem) corev1.PersistentVolumeAccessMode {
	if mapping != nil && mapping.AccessMode != nil {
		return *mapping.AccessMode
	}
	accessMode := corev1.ReadWriteOnce
	if readOnly, ok := diskAttachment.ReadOnly(); ok && readOnly {
		accessMode = corev1.ReadOnlyMany
	}
	if pp, ok := o.vm.PlacementPolicy(); ok {
		if affinity, _ := pp.Affinity(); affinity == ovirtsdk.VMAFFINITY_MIGRATABLE {
			accessMode = corev1.ReadWriteMany
		}
	}

	return accessMode
}

// MapDataVolumes map the oVirt VM disks to the map of CDI DataVolumes specification, where
// map key is the target-vm-name + id of the oVirt disk
func (o *OvirtMapper) MapDataVolumes(targetVMName *string, filesystemOverhead cdiv1.FilesystemOverhead) (map[string]cdiv1.DataVolume, error) {
	// TODO: stateless, boot_devices, floppy/cdrom
	diskAttachments, _ := o.vm.DiskAttachments()
	dvs := make(map[string]cdiv1.DataVolume, len(diskAttachments.Slice()))

	for _, diskAttachment := range diskAttachments.Slice() {
		diskAttachID, _ := diskAttachment.Id()
		dvName := buildDataVolumeName(*targetVMName, diskAttachID)
		disk, _ := diskAttachment.Disk()
		diskID, _ := disk.Id()

		mapping := o.getMapping(disk, o.mappings)
		accessMode := o.getAccessMode(diskAttachment, mapping)
		sdClass := o.getStorageClassForDisk(mapping)
		volumeMode := o.getVolumeMode(mapping)

		diskSize, _ := disk.ProvisionedSize()
		overhead := utils.GetOverheadForStorageClass(filesystemOverhead, sdClass)

		blockSize := int64(512)
		sizeWithOverhead := utils.RoundUp(int64(float64(diskSize)/(1-overhead)), blockSize)
		diskSizeConverted, err := utils.FormatBytes(sizeWithOverhead)
		if err != nil {
			return dvs, err
		}
		quantity, _ := resource.ParseQuantity(diskSizeConverted)

		dvs[dvName] = cdiv1.DataVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: cdiAPIVersion,
				Kind:       dataVolumeKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dvName,
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
						accessMode,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: quantity,
						},
					},
					VolumeMode: volumeMode,
				},
			},
		}
		if sdClass != nil {
			dvs[dvName].Spec.PVC.StorageClassName = sdClass
		}
	}
	return dvs, nil
}

func (o *OvirtMapper) mapNics(networkToType map[string]string) []kubevirtv1.Interface {
	var kubevirtNics []kubevirtv1.Interface
	nics, _ := o.vm.Nics()
	for _, nic := range nics.Slice() {
		sriov := false

		// This network interface doesn't have any vnic profile specified.
		if vNicProfile, ok := nic.VnicProfile(); ok {
			sriov = outils.IsSRIOV(vNicProfile)
		} else {
			continue
		}

		kubevirtNic := kubevirtv1.Interface{}
		nicName, _ := nic.Name()
		kubevirtNic.Name, _ = utils.NormalizeName(nicName)
		if nicMac, ok := nic.Mac(); ok {
			kubevirtNic.MacAddress, _ = nicMac.Address()
		}
		if nicInterface, ok := nic.Interface(); ok && !sriov {
			kubevirtNic.Model = string(nicInterface)
		}

		switch networkToType[kubevirtNic.Name] {
		case networkTypeMultus:
			if sriov {
				kubevirtNic.SRIOV = &kubevirtv1.InterfaceSRIOV{}
			} else {
				kubevirtNic.Bridge = &kubevirtv1.InterfaceBridge{}
			}
		case networkTypePod:
			kubevirtNic.Masquerade = &kubevirtv1.InterfaceMasquerade{}
		}

		kubevirtNics = append(kubevirtNics, kubevirtNic)
	}

	return kubevirtNics
}

func (o *OvirtMapper) mapNetworks() []kubevirtv1.Network {
	var kubevirtNetworks []kubevirtv1.Network
	nics, _ := o.vm.Nics()
	for _, nic := range nics.Slice() {
		// This network interface don't have any network specified.
		nicProfile, ok := nic.VnicProfile()
		if !ok {
			continue
		}

		kubevirtNet := o.getNetworkForNic(nicProfile)
		nicName, _ := nic.Name()
		kubevirtNet.Name, _ = utils.NormalizeName(nicName)
		kubevirtNetworks = append(kubevirtNetworks, kubevirtNet)
	}

	return kubevirtNetworks
}

func (o *OvirtMapper) mapNetworksToTypes(networks []kubevirtv1.Network) map[string]string {
	networkToType := make(map[string]string)
	for _, network := range networks {
		if network.Multus != nil {
			networkToType[network.Name] = networkTypeMultus
		} else if network.Pod != nil {
			networkToType[network.Name] = networkTypePod
		}
	}
	return networkToType
}

func (o *OvirtMapper) getNetworkForNic(vnicProfile *ovirtsdk.VnicProfile) kubevirtv1.Network {
	kubevirtNet := kubevirtv1.Network{}
	network, _ := vnicProfile.Network()
	nicNetworkName, _ := network.Name()
	vnicProfileName, _ := vnicProfile.Name()
	sriov := outils.IsSRIOV(vnicProfile)
	nicMappingName := outils.GetNetworkMappingName(nicNetworkName, vnicProfileName)
	for _, mapping := range *o.mappings.NetworkMappings {
		if mapping.Source.Name != nil && nicMappingName == *mapping.Source.Name {
			o.mapNetworkType(mapping, &kubevirtNet, sriov)
		}
		if mapping.Source.ID != nil {
			if vnicProfileID, _ := vnicProfile.Id(); vnicProfileID == *mapping.Source.ID {
				o.mapNetworkType(mapping, &kubevirtNet, sriov)
			}
		}
	}
	return kubevirtNet
}

func (o *OvirtMapper) mapNetworkType(mapping v2vv1.NetworkResourceMappingItem, kubevirtNet *kubevirtv1.Network, sriov bool) {
	if mapping.Type == nil || *mapping.Type == networkTypePod {
		kubevirtNet.Pod = &kubevirtv1.PodNetwork{}
	} else if *mapping.Type == networkTypeMultus {
		var name string
		if sriov {
			namespace := o.namespace
			if mapping.Target.Namespace != nil {
				namespace = *mapping.Target.Namespace
			}
			name = namespace + "/" + mapping.Target.Name
		} else {
			name = mapping.Target.Name
		}
		kubevirtNet.Multus = &kubevirtv1.MultusNetwork{
			NetworkName: name,
		}
	}
}

// ResolveVMName resolves the target VM name
func (o *OvirtMapper) ResolveVMName(targetVMName *string) *string {
	vmNameBase := o.resolveVMNameBase(targetVMName)
	if vmNameBase == nil {
		return nil
	}
	// VM name is put in label values and has to be shorter than regular k8s name
	// https://bugzilla.redhat.com/1857165
	name := utils.EnsureLabelValueLength(*vmNameBase)
	return &name
}

func (o *OvirtMapper) resolveVMNameBase(targetVMName *string) *string {
	if targetVMName != nil {
		return targetVMName
	}

	name, ok := o.vm.Name()
	if !ok {
		return nil
	}

	name, err := utils.NormalizeName(name)
	if err != nil {
		// TODO: should name validation be included in condition ?
		return nil
	}

	return &name
}

func (o *OvirtMapper) getMapping(disk *ovirtsdk.Disk, mappings *v2vv1.OvirtMappings) *v2vv1.StorageResourceMappingItem {
	if mappings.DiskMappings != nil {
		for _, mapping := range *mappings.DiskMappings {
			if mapping.Source.ID != nil {
				if id, _ := disk.Id(); id == *mapping.Source.ID {
					return &mapping
				}
			}
			if mapping.Source.Name != nil {
				if name, _ := disk.Alias(); name == *mapping.Source.Name {
					return &mapping
				}
			}
		}
	}

	if mappings.StorageMappings != nil {
		sd, _ := disk.StorageDomain()
		for _, mapping := range *mappings.StorageMappings {
			if mapping.Source.ID != nil {
				if id, _ := sd.Id(); id == *mapping.Source.ID {
					return &mapping
				}
			}
			if mapping.Source.Name != nil {
				if name, _ := sd.Name(); name == *mapping.Source.Name {
					return &mapping
				}
			}
		}
	}

	return nil
}

func (o *OvirtMapper) getVolumeMode(mapping *v2vv1.StorageResourceMappingItem) *corev1.PersistentVolumeMode {
	if mapping != nil {
		return mapping.VolumeMode
	}

	return &DefaultVolumeMode
}

func (o *OvirtMapper) getStorageClassForDisk(mapping *v2vv1.StorageResourceMappingItem) *string {
	if mapping != nil {
		targetName := mapping.Target.Name
		if targetName != DefaultStorageClassTargetName {
			return &targetName
		}
	}

	// Use default storage class:
	return nil
}

func (o *OvirtMapper) mapArchitecture() *kubevirtv1.Machine {
	cpu, _ := o.vm.Cpu()
	arch, _ := cpu.Architecture()
	machine := &kubevirtv1.Machine{
		Type: archMapping[string(arch)],
	}

	// Override machine type in case custom emulated machine is specified
	if custom, ok := o.vm.CustomEmulatedMachine(); ok {
		// do not override when i440fx which is unsupported
		if !strings.Contains(custom, "i440fx") {
			machine.Type = custom
		}
	}

	return machine
}

func (o *OvirtMapper) mapFirmware() *kubevirtv1.Firmware {
	firmware := &kubevirtv1.Firmware{}

	// Map bootloader
	firmware.Bootloader = BiosTypeMapping[getBiosType(o.vm)]

	// Map serial number
	serial := o.mapSerialNumber()
	if serial != nil {
		firmware.Serial = *serial
	}

	return firmware
}

func getBiosType(vm *ovirtsdk.Vm) string {
	bios, _ := vm.Bios()
	biosType, _ := bios.Type()
	if biosType == ovirtsdk.BIOSTYPE_CLUSTER_DEFAULT {
		if cluster, ok := vm.Cluster(); ok {
			if clusterDefaultBiosType, ok := cluster.BiosType(); ok {
				biosType = clusterDefaultBiosType
			}
		}
	}
	return string(biosType)
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
		if tune, ok := cpuDef.CpuTune(); ok {
			if outils.IsCPUPinningExact(tune) {
				cpu.DedicatedCPUPlacement = true
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
	memory := &kubevirtv1.Memory{}

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

func (o *OvirtMapper) mapResourceRequirements(dedicatedCPUPlacement bool) (kubevirtv1.ResourceRequirements, error) {
	reqs := kubevirtv1.ResourceRequirements{}

	// Requests
	memoryPolicy, _ := o.vm.MemoryPolicy()
	maxMemory, _ := memoryPolicy.Max()
	maxMemoryConverted, err := utils.FormatBytes(maxMemory)
	if err != nil {
		return reqs, err
	}
	maxMemoryQuantity, _ := resource.ParseQuantity(maxMemoryConverted)
	requestedMemory := maxMemoryQuantity
	if !dedicatedCPUPlacement {
		ovirtVMMemory, _ := o.vm.Memory()
		vmMemoryConverted, err := utils.FormatBytes(ovirtVMMemory)
		if err != nil {
			return reqs, err
		}
		requestedMemory, _ = resource.ParseQuantity(vmMemoryConverted)
	}

	reqs.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceMemory: requestedMemory,
	}

	return reqs, nil
}

// Graphical console is attached only in case the VM is not in headless mode
func (o *OvirtMapper) mapGraphicalConsoles(os string) *kubevirtv1.Devices {
	// GraphicsConsole
	vncEnabled := true
	if gc, _ := o.vm.GraphicsConsoles(); len(gc.Slice()) == 0 {
		vncEnabled = false
	}
	devices := &kubevirtv1.Devices{}
	if vncEnabled {
		devices.AutoattachGraphicsDevice = &vncEnabled
		devices.Inputs = []kubevirtv1.Input{o.prepareTabletInputDevice(os)}
	}

	// SerialConsole
	if console, ok := o.vm.Console(); ok {
		consoleEnabled, _ := console.Enabled()
		devices.AutoattachSerialConsole = &consoleEnabled
	}

	return devices
}

func (o *OvirtMapper) prepareTabletInputDevice(os string) kubevirtv1.Input {
	tablet := kubevirtv1.Input{
		Type: "tablet",
		Name: "tablet",
	}

	if len(os) >= 3 && strings.EqualFold(os[:3], "win") {
		tablet.Bus = "usb"
	} else {
		tablet.Bus = "virtio"
	}
	return tablet
}

func (o *OvirtMapper) mapLabels(vmLabels map[string]string) map[string]string {
	var labels map[string]string
	if vmLabels == nil {
		labels = map[string]string{}
	} else {
		labels = vmLabels
	}
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
	if placementPolicy, ok := o.vm.PlacementPolicy(); ok {
		if affinity, ok := placementPolicy.Affinity(); ok {
			if affinity == ovirtsdk.VMAFFINITY_MIGRATABLE {
				strategy := kubevirtv1.EvictionStrategyLiveMigrate
				return &strategy
			}
		}
	}

	return nil
}

func (o *OvirtMapper) mapFeatures() *kubevirtv1.Features {
	features := &kubevirtv1.Features{}

	bootloader := BiosTypeMapping[getBiosType(o.vm)]

	// Enabling EFI will also enable Secure Boot, which requires SMM to be enabled.
	if bootloader.EFI != nil {
		features.SMM = &kubevirtv1.FeatureState{
			Enabled: &_true,
		}
	}

	return features
}

func (o *OvirtMapper) mapTimeZone() *kubevirtv1.Clock {
	clock := kubevirtv1.Clock{Timer: &kubevirtv1.Timer{}}
	offset := kubevirtv1.ClockOffsetUTC{}

	if tz, ok := o.vm.TimeZone(); ok {
		if utcOffset, ok := tz.UtcOffset(); ok {
			parsedOffset, err := utils.ParseUtcOffsetToSeconds(utcOffset)
			if err == nil {
				offset.OffsetSeconds = &parsedOffset
			} else {
				log.Info("VM's utc offset is malformed: " + err.Error())
			}
		} else if tzName, ok := tz.Name(); ok {
			timezone := kubevirtv1.ClockOffsetTimezone(tzName)
			clock.Timezone = &timezone
		}
	}

	if clock.Timezone == nil {
		clock.UTC = &offset
	}

	return &clock
}

func getDiskAttachmentByID(id string, diskAttachments *ovirtsdk.DiskAttachmentSlice, targetVMName string) *ovirtsdk.DiskAttachment {
	for _, diskAttachment := range diskAttachments.Slice() {
		if diskID, ok := diskAttachment.Id(); ok && buildDataVolumeName(targetVMName, diskID) == id {
			return diskAttachment
		}
	}
	return nil
}

func buildDataVolumeName(targetVMName string, diskAttachID string) string {
	sha := sha1.New()
	sha.Write([]byte(targetVMName + diskAttachID))
	return fmt.Sprintf("%x", sha.Sum(nil))
}
