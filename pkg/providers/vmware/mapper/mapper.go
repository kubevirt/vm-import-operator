package mapper

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	v1beta1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	vos "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	cdiAPIVersion                 = "cdi.kubevirt.io/v1alpha1"
	dataVolumeKind                = "DataVolume"
	defaultStorageClassTargetName = ""
	labelTag                      = "tags"
	vmNamePrefix                  = "vmware-"
	vmwareDescription             = "vmware-description"
)

// bus types
const (
	busTypeUSB    = "usb"
	busTypeVirtio = "virtio"
)

// network types
const (
	networkTypeMultus = "multus"
	networkTypePod    = "pod"
)

// architectures
const (
	q35 = "q35"
)

var (
	defaultVolumeMode = corev1.PersistentVolumeFilesystem
	defaultAccessMode = corev1.ReadWriteOnce
)

var biosTypeMapping = map[string]*kubevirtv1.Bootloader{
	"efi":  {EFI: &kubevirtv1.EFI{}},
	"bios": {BIOS: &kubevirtv1.BIOS{}},
}

// DataVolumeCredentials defines the credentials required
// for creating a DataVolume with a VDDK source.
type DataVolumeCredentials struct {
	URL        string
	Username   string
	Password   string
	Thumbprint string
	SecretName string
}


// Disk is an abstraction of a VMWare VirtualDisk
type Disk struct {
	BackingFileName string
	Capacity        int64
	DatastoreMoRef  string
	DatastoreName   string
	ID              string
	Name            string
	Key             int32
}

// Nic is an abstraction of a VMWare VirtualEthernetCard
type Nic struct {
	Name        string
	Network     string
	Mac         string
	DVPortGroup string
}

// BuildDisks retrieves each of the VM's VirtualDisks
// and pulls out the values that are needed for import
func BuildDisks(vmProperties *mo.VirtualMachine) []Disk {
	disks := make([]Disk, 0)

	devices := vmProperties.Config.Hardware.Device
	for _, device := range devices {
		// is this device a VirtualDisk?
		if virtualDisk, ok := device.(*types.VirtualDisk); ok {
			var datastoreMoRef string
			var datastoreName string
			var backingFileName string
			var diskId string

			backing := virtualDisk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
			backingInfo := backing.GetVirtualDeviceFileBackingInfo()
			if backingInfo.Datastore != nil {
				datastoreMoRef = backingInfo.Datastore.Value
			}
			backingFileName = backingInfo.FileName
			datastoreName = getDatastoreNameFromBacking(backingFileName)

			if virtualDisk.VDiskId != nil {
				diskId = virtualDisk.VDiskId.Id
			} else {
				diskId = virtualDisk.DiskObjectId
			}

			disk := Disk{
				BackingFileName: backingFileName,
				Capacity:        getDiskCapacityInBytes(virtualDisk),
				DatastoreMoRef:  datastoreMoRef,
				DatastoreName:   datastoreName,
				ID:              diskId,
				Key:             virtualDisk.Key,
				Name:            virtualDisk.DeviceInfo.GetDescription().Label,
			}

			disks = append(disks, disk)
			continue
		}

	}

	return disks
}

// BuildNics retrieves each of the VM's VirtualEthernetCards
// and pulls out the values that are needed for import
func BuildNics(vmProperties *mo.VirtualMachine) []Nic {
	nics := make([]Nic, 0)

	devices := vmProperties.Config.Hardware.Device
	for _, device := range devices {
		// is this device a VirtualEthernetCard?
		var virtualNetwork *types.VirtualEthernetCard
		switch v := device.(type) {
		case *types.VirtualE1000:
			virtualNetwork = &v.VirtualEthernetCard
		case *types.VirtualE1000e:
			virtualNetwork = &v.VirtualEthernetCard
		case *types.VirtualVmxnet:
			virtualNetwork = &v.VirtualEthernetCard
		case *types.VirtualVmxnet2:
			virtualNetwork = &v.VirtualEthernetCard
		case *types.VirtualVmxnet3:
			virtualNetwork = &v.VirtualEthernetCard
		}
		if virtualNetwork != nil && virtualNetwork.Backing != nil {
			var network string
			var dvportgroup string
			var name string

			switch backing := virtualNetwork.Backing.(type) {
			case *types.VirtualEthernetCardNetworkBackingInfo:
				if backing.Network != nil {
					network = backing.Network.Value
				}
				// despite being called DeviceName, this is actually
				// the name of the Network the device is attached to,
				// e.g. "VM Network"
				name = backing.DeviceName
			case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
				dvportgroup = backing.Port.PortgroupKey
				desc := virtualNetwork.DeviceInfo.GetDescription()
				if desc != nil {
					// this is the actual device name, e.g. "ethernet-0"
					name = desc.Label
				}
			}

			nic := Nic{
				Name:        name,
				Mac:         virtualNetwork.MacAddress,
				Network:     network,
				DVPortGroup: dvportgroup,
			}
			nics = append(nics, nic)
		}
	}
	return nics
}

// VmwareMapper is a struct that holds attributes needed to map a vSphere VM to Kubevirt
type VmwareMapper struct {
	credentials    *DataVolumeCredentials
	disks          *[]Disk
	hostProperties *mo.HostSystem
	instanceUID    string
	mappings       *v1beta1.VmwareMappings
	namespace      string
	nics           *[]Nic
	osFinder       vos.OSFinder
	vm             *object.VirtualMachine
	vmProperties   *mo.VirtualMachine
}

// NewVmwareMapper creates a new VmwareMapper struct
func NewVmwareMapper(vm *object.VirtualMachine, vmProperties *mo.VirtualMachine, hostProperties *mo.HostSystem, credentials *DataVolumeCredentials, mappings *v1beta1.VmwareMappings, instanceUID string, namespace string, osFinder vos.OSFinder) *VmwareMapper {
	return &VmwareMapper{
		credentials:    credentials,
		hostProperties: hostProperties,
		instanceUID:    instanceUID,
		mappings:       mappings,
		namespace:      namespace,
		osFinder:       osFinder,
		vm:             vm,
		vmProperties:   vmProperties,
	}
}

// buildNics retrieves each of the VM's VirtualEthernetCards
// and pulls out the values that are needed for import
func (r *VmwareMapper) buildNics() {
	if r.nics != nil {
		return
	}
	nics := BuildNics(r.vmProperties)
	r.nics = &nics
}

// buildDisks retrieves each of the VM's VirtualDisks
// and pulls out the values that are needed for import
func (r *VmwareMapper) buildDisks() error {
	if r.disks != nil {
		return nil
	}
	disks := BuildDisks(r.vmProperties)
	r.disks = &disks
	return nil
}

func (r *VmwareMapper) getMappingForDisk(disk Disk) *v1beta1.StorageResourceMappingItem {
	if r.mappings.DiskMappings != nil {
		for _, mapping := range *r.mappings.DiskMappings {
			if mapping.Source.ID != nil {
				if disk.ID == *mapping.Source.ID {
					return &mapping
				}
			}
			if mapping.Source.Name != nil {
				if disk.Name == *mapping.Source.Name {
					return &mapping
				}
			}
		}
	}

	if r.mappings.StorageMappings != nil {
		for _, mapping := range *r.mappings.StorageMappings {
			if mapping.Source.ID != nil {
				if disk.DatastoreMoRef == *mapping.Source.ID {
					return &mapping
				}
			}
			if mapping.Source.Name != nil {
				if disk.DatastoreName == *mapping.Source.Name {
					return &mapping
				}
			}
		}
	}
	return nil
}

func (r *VmwareMapper) getStorageClassForDisk(mapping *v1beta1.StorageResourceMappingItem) *string {
	if mapping != nil {
		targetName := mapping.Target.Name
		if targetName != defaultStorageClassTargetName {
			return &targetName
		}
	}

	// Use default storage class:
	return nil
}

func (r *VmwareMapper) getAccessModeForDisk(mapping *v1beta1.StorageResourceMappingItem) corev1.PersistentVolumeAccessMode {
	if mapping != nil && mapping.AccessMode != nil {
		return *mapping.AccessMode
	}

	return defaultAccessMode
}

func (r *VmwareMapper) getVolumeModeForDisk(mapping *v1beta1.StorageResourceMappingItem) *corev1.PersistentVolumeMode {
	if mapping != nil && mapping.VolumeMode != nil {
		return mapping.VolumeMode
	}

	return &defaultVolumeMode
}

// MapDataVolumes maps the VMware disks to CDI DataVolumes
func (r *VmwareMapper) MapDataVolumes(_ *string, filesystemOverhead cdiv1.FilesystemOverhead) (map[string]cdiv1.DataVolume, error) {
	err := r.buildDisks()
	if err != nil {
		return nil, err
	}

	dvs := make(map[string]cdiv1.DataVolume)

	for _, disk := range *r.disks {
		dvName := fmt.Sprintf("%s-%d", r.instanceUID, disk.Key)

		mapping := r.getMappingForDisk(disk)

		storageClass := r.getStorageClassForDisk(mapping)

		overhead := utils.GetOverheadForStorageClass(filesystemOverhead, storageClass)
		capacityWithOverhead := (int64(float64(disk.Capacity) / (1 - overhead)) / 512 + 1 ) * 512
		capacityAsQuantity, err := bytesToQuantity(capacityWithOverhead)
		if err != nil {
			return nil, err
		}

		dvs[dvName] = cdiv1.DataVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: cdiAPIVersion,
				Kind:       dataVolumeKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dvName,
				Namespace: r.namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: cdiv1.DataVolumeSource{
					VDDK: &cdiv1.DataVolumeSourceVDDK{
						URL:         r.credentials.URL,
						UUID:        r.vmProperties.Config.Uuid,
						BackingFile: disk.BackingFileName,
						Thumbprint:  r.credentials.Thumbprint,
						SecretRef:   r.credentials.SecretName,
					},
				},
				PVC: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						r.getAccessModeForDisk(mapping),
					},
					VolumeMode: r.getVolumeModeForDisk(mapping),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: capacityAsQuantity,
						},
					},
					StorageClassName: storageClass,
				},
			},
		}
	}
	return dvs, nil
}

// MapDisk maps a disk from the VMware VM to the Kubevirt VM.
func (r *VmwareMapper) MapDisk(vmSpec *kubevirtv1.VirtualMachine, dv cdiv1.DataVolume) {
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

	kubevirtDisk := kubevirtv1.Disk{
		Name: name,
		DiskDevice: kubevirtv1.DiskDevice{
			Disk: &kubevirtv1.DiskTarget{
				Bus: busTypeVirtio,
			},
		},
	}

	vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, volume)
	disks := append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, kubevirtDisk)

	// Since the import controller is iterating over a map of DVs,
	// MapDisk gets called for each DV in a nondeterministic order which results
	// in the disks being in an arbitrary order. This sort ensure the disks are
	// attached in the same order as the devices on the source VM.
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Name < disks[j].Name
	})
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = disks
}

// ResolveVMName resolves the target VM name
func (r *VmwareMapper) ResolveVMName(targetVMName *string) *string {
	vmNameBase := r.resolveVMNameBase(targetVMName)
	if vmNameBase == nil {
		return nil
	}
	// VM name is put in label values and has to be shorter than regular k8s name
	// https://bugzilla.redhat.com/1857165
	name := utils.EnsureLabelValueLength(*vmNameBase)
	return &name
}

func (r *VmwareMapper) resolveVMNameBase(targetVMName *string) *string {
	if targetVMName != nil {
		return targetVMName
	}

	name, err := utils.NormalizeName(r.vm.Name())
	if err != nil {
		return nil
	}

	return &name
}

// CreateEmptyVM creates an empty Kubevirt VM
func (r *VmwareMapper) CreateEmptyVM(vmName *string) *kubevirtv1.VirtualMachine {
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

// MapVM maps resources from a VMware VM to a Kubevirt VM
func (r *VmwareMapper) MapVM(targetVmName *string, vmSpec *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	if vmSpec.Spec.Template == nil {
		vmSpec.Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{}
	}
	// Map annotations
	vmSpec.ObjectMeta.Annotations = r.mapAnnotations()
	// Map labels like vm tags
	vmSpec.ObjectMeta.Labels = r.mapLabels(vmSpec.ObjectMeta.Labels)
	// Set Namespace
	vmSpec.ObjectMeta.Namespace = r.namespace

	// Map name
	if targetVmName == nil {
		vmSpec.ObjectMeta.GenerateName = vmNamePrefix
	} else {
		vmSpec.ObjectMeta.Name = *targetVmName
	}

	if vmSpec.Spec.Template == nil {
		vmSpec.Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{}
	}

	true_ := true
	false_ := false
	vmSpec.Spec.Running = &false_

	// Map hostname
	hostname, _ := utils.NormalizeName(r.vmProperties.Guest.HostName)
	// if this is a FQDN, split off the first subdomain and use it as the hostname.
	nameParts := strings.Split(hostname, ".")
	vmSpec.Spec.Template.Spec.Hostname = nameParts[0]

	vmSpec.Spec.Template.Spec.Domain.Machine = kubevirtv1.Machine{Type: q35}
	vmSpec.Spec.Template.Spec.Domain.CPU = r.mapCPUTopology()
	vmSpec.Spec.Template.Spec.Domain.Firmware = r.mapFirmware()
	vmSpec.Spec.Template.Spec.Domain.Features = r.mapFeatures()
	reservations, err := r.mapResourceReservations()
	if err != nil {
		return nil, err
	}
	vmSpec.Spec.Template.Spec.Domain.Resources = reservations

	// Map clock
	vmSpec.Spec.Template.Spec.Domain.Clock = r.mapClock(r.hostProperties)

	// remove any default networks/interfaces from the template
	vmSpec.Spec.Template.Spec.Networks = []kubevirtv1.Network{}
	vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{}

	if r.mappings != nil && r.mappings.NetworkMappings != nil {
		// Map networks
		vmSpec.Spec.Template.Spec.Networks, err = r.mapNetworks()
		if err != nil {
			return nil, err
		}

		networkToType := r.mapNetworksToTypes(vmSpec.Spec.Template.Spec.Networks)
		vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces, err = r.mapNetworkInterfaces(networkToType)
		if err != nil {
			return nil, err
		}
	}

	// if there are no interfaces defined, force NetworkInterfaceMultiQueue to false
	// https://github.com/kubevirt/common-templates/issues/186
	if len(vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces) > 0 {
		vmSpec.Spec.Template.Spec.Domain.Devices.NetworkInterfaceMultiQueue = &true_
	} else {
		vmSpec.Spec.Template.Spec.Domain.Devices.NetworkInterfaceMultiQueue = &false_
	}

	os, _ := r.osFinder.FindOperatingSystem(r.vmProperties)
	vmSpec.Spec.Template.Spec.Domain.Devices.Inputs = r.mapInputDevice(os)
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = []kubevirtv1.Disk{}
	return vmSpec, nil
}

func (r *VmwareMapper) mapLabels(vmLabels map[string]string) map[string]string {
	var labels map[string]string
	if vmLabels == nil {
		labels = map[string]string{}
	} else {
		labels = vmLabels
	}

	var tagList []string
	for _, tag := range r.vmProperties.Tag {
		tagList = append(tagList, tag.Key)
	}
	labels[labelTag] = strings.Join(tagList, ",")
	return labels
}

func (r *VmwareMapper) mapAnnotations() map[string]string {
	annotations := map[string]string{}
	annotations[vmwareDescription] = r.vmProperties.Config.Annotation
	return annotations
}

func (r *VmwareMapper) mapClock(hostProperties *mo.HostSystem) *kubevirtv1.Clock {
	offset := &kubevirtv1.ClockOffsetUTC{}
	if hostProperties.Config != nil && hostProperties.Config.DateTimeInfo != nil {
		offsetSeconds := int(hostProperties.Config.DateTimeInfo.TimeZone.GmtOffset)
		offset.OffsetSeconds = &offsetSeconds
	}
	clock := &kubevirtv1.Clock{Timer: &kubevirtv1.Timer{}}
	clock.UTC = offset
	return clock
}

func (r *VmwareMapper) mapCPUTopology() *kubevirtv1.CPU {
	cpu := &kubevirtv1.CPU{}
	numSockets := r.vmProperties.Config.Hardware.NumCPU / r.vmProperties.Config.Hardware.NumCoresPerSocket
	cpu.Sockets = uint32(numSockets)
	cpu.Cores = uint32(r.vmProperties.Config.Hardware.NumCoresPerSocket)
	return cpu
}

func (r *VmwareMapper) mapFeatures() *kubevirtv1.Features {
	features := &kubevirtv1.Features{}
	bootloader := biosTypeMapping[r.vmProperties.Config.Firmware]
	if bootloader != nil && bootloader.EFI != nil {
		// Enabling EFI will also enable Secure Boot, which requires SMM to be enabled.
		smmEnabled := true
		features.SMM = &kubevirtv1.FeatureState{
			Enabled: &smmEnabled,
		}
	}

	return features
}

func (r *VmwareMapper) mapFirmware() *kubevirtv1.Firmware {
	firmwareSpec := &kubevirtv1.Firmware{}
	firmwareSpec.Bootloader = biosTypeMapping[r.vmProperties.Config.Firmware]
	if firmwareSpec.Bootloader == nil {
		firmwareSpec.Bootloader = biosTypeMapping["bios"]
	}
	firmwareSpec.Serial = r.vmProperties.Config.InstanceUuid
	return firmwareSpec
}

func (r *VmwareMapper) mapInputDevice(os string) []kubevirtv1.Input {
	tablet := kubevirtv1.Input{
		Type: "tablet",
		Name: "tablet",
	}

	if len(os) >= 3 && strings.EqualFold(os[:3], "win") {
		tablet.Bus = busTypeUSB
	} else {
		tablet.Bus = busTypeVirtio
	}
	return []kubevirtv1.Input{tablet}
}

func (r *VmwareMapper) mapNetworks() ([]kubevirtv1.Network, error) {
	r.buildNics()

	var kubevirtNetworks []kubevirtv1.Network
	for _, nic := range *r.nics {
		kubevirtNet := kubevirtv1.Network{}
		for _, mapping := range *r.mappings.NetworkMappings {
			if (mapping.Source.Name != nil && nic.Name == *mapping.Source.Name) ||
				(mapping.Source.ID != nil && (nic.Network == *mapping.Source.ID || nic.DVPortGroup == *mapping.Source.ID)) {
				if mapping.Type == nil || *mapping.Type == networkTypePod {
					kubevirtNet.Pod = &kubevirtv1.PodNetwork{}
				} else if *mapping.Type == networkTypeMultus {
					kubevirtNet.Multus = &kubevirtv1.MultusNetwork{
						NetworkName: mapping.Target.Name,
					}
				}
				kubevirtNet.Name, _ = utils.NormalizeName(nic.Name)
				kubevirtNetworks = append(kubevirtNetworks, kubevirtNet)
			}
		}
	}

	return kubevirtNetworks, nil
}

func (r *VmwareMapper) mapNetworkInterfaces(networkToType map[string]string) ([]kubevirtv1.Interface, error) {
	r.buildNics()
	var interfaces []kubevirtv1.Interface
	for _, nic := range *r.nics {
		kubevirtInterface := kubevirtv1.Interface{}
		kubevirtInterface.MacAddress = nic.Mac
		kubevirtInterface.Name, _ = utils.NormalizeName(nic.Name)
		kubevirtInterface.Model = "virtio"
		switch networkToType[kubevirtInterface.Name] {
		case networkTypeMultus:
			kubevirtInterface.Bridge = &kubevirtv1.InterfaceBridge{}
			interfaces = append(interfaces, kubevirtInterface)
		case networkTypePod:
			kubevirtInterface.Masquerade = &kubevirtv1.InterfaceMasquerade{}
			interfaces = append(interfaces, kubevirtInterface)
		}
	}

	return interfaces, nil
}

func (r *VmwareMapper) mapNetworksToTypes(networks []kubevirtv1.Network) map[string]string {
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

func (r *VmwareMapper) mapResourceReservations() (kubevirtv1.ResourceRequirements, error) {
	reqs := kubevirtv1.ResourceRequirements{}

	reservation := int64(r.vmProperties.Summary.Config.MemorySizeMB)
	resString := strconv.FormatInt(reservation, 10) + "Mi"
	resQuantity, err := resource.ParseQuantity(resString)
	if err != nil {
		return reqs, err
	}
	reqs.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceMemory: resQuantity,
	}
	return reqs, nil
}

func bytesToQuantity(bytes int64) (resource.Quantity, error) {
	var capacity resource.Quantity

	diskSizeConverted, err := utils.FormatBytes(bytes)
	if err != nil {
		return capacity, err
	}
	capacity, err = resource.ParseQuantity(diskSizeConverted)
	if err != nil {
		return capacity, err
	}
	return capacity, nil
}

func getDiskCapacityInBytes(disk *types.VirtualDisk) int64 {
	var capacityInBytes int64

	if disk.CapacityInBytes > 0 {
		capacityInBytes = disk.CapacityInBytes
	} else {
		capacityInBytes = disk.CapacityInKB * 1024
	}

	return capacityInBytes
}

func getDatastoreNameFromBacking(backingFile string) string {
	var datastoreName string
	datastoreNamePattern := regexp.MustCompile(`^\[(.+)\]`)
	matches := datastoreNamePattern.FindStringSubmatch(backingFile)
	if len(matches) > 1 {
		datastoreName = matches[1]
	}
	return datastoreName
}
