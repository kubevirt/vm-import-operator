package mapper

import (
	"fmt"
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
	DefaultStorageClassTargetName = ""
	LabelTag                      = "tags"
	vmNamePrefix                  = "vmware-"
	vmwareDescription             = "vmware-description"
)

// bus types
const (
	busTypeSata   = "sata"
	busTypeSCSI   = "scsi"
	busTypeUSB    = "usb"
	busTypeVirtio = "virtio"
)

// network types
const (
	networkTypeMultus = "multus"
	networkTypePod    = "pod"
)

var biosTypeMapping = map[string]*kubevirtv1.Bootloader{
	"efi":  {EFI: &kubevirtv1.EFI{}},
	"bios": {BIOS: &kubevirtv1.BIOS{}},
}

// DataVolumeCredentials defines the credentials required
// for creating a DataVolume with a VDDK source.
type DataVolumeCredentials struct {
	URL string
	Username string
	Password string
	Thumbprint string
	SecretName string
}

// disk is an abstraction of a VMWare VirtualDisk
type disk struct {
	backingFileName string
	bus             string
	capacity        resource.Quantity
	datastore       string
	id              string
	name            string
}

// network is an abstraction of a VMWare VirtualEthernetCard
type network struct {
	name string
	mac string
}

type VmwareMapper struct {
	credentials    *DataVolumeCredentials
	disks          *[]disk
	disksByDVName  map[string]disk
	hostProperties *mo.HostSystem
	mappings       *v1beta1.VmwareMappings
	namespace      string
	networks       *[]network
	osFinder       vos.OSFinder
	vm             *object.VirtualMachine
	vmProperties   *mo.VirtualMachine
}


func NewVmwareMapper(vm *object.VirtualMachine, vmProperties *mo.VirtualMachine, hostProperties *mo.HostSystem, credentials *DataVolumeCredentials, mappings *v1beta1.VmwareMappings, namespace string, osFinder vos.OSFinder) *VmwareMapper {
	return &VmwareMapper{
		credentials:    credentials,
		disksByDVName:  make(map[string]disk),
		hostProperties: hostProperties,
		mappings:       mappings,
		namespace:      namespace,
		osFinder:       osFinder,
		vm:             vm,
		vmProperties:   vmProperties,
	}
}

// buildDevices retrieves each of the VM's VirtualDisks and VirtualEthernetCards
// and pulls out the values that are needed for import
func (r *VmwareMapper) buildDevices() error {
	if r.disks != nil && r.networks != nil {
		return nil
	}

	disks := make([]disk, 0)
	networks := make([]network, 0)

	devices := r.vmProperties.Config.Hardware.Device
	for _, device := range devices {
		// the device is a disk
		if virtualDisk, ok := device.(*types.VirtualDisk); ok {
			var datastore string
			var backingFileName string
			switch backing := virtualDisk.Backing.(type) {
			case types.BaseVirtualDeviceFileBackingInfo:

				backingInfo := backing.GetVirtualDeviceFileBackingInfo()
				if backingInfo.Datastore != nil {
					datastore = backingInfo.Datastore.Value
				}
				backingFileName = backingInfo.FileName
			}

			capacity, err := getCapacityForVirtualDisk(virtualDisk)
			if err != nil {
				return err
			}

			var diskId string
			if virtualDisk.VDiskId != nil {
				diskId = virtualDisk.VDiskId.Id
			} else {
				diskId = virtualDisk.DiskObjectId
			}

			disk := disk{
				backingFileName: backingFileName,
				bus:             busTypeVirtio,
				capacity:        capacity,
				datastore:       datastore,
				id:              diskId,
				name:            virtualDisk.DeviceInfo.GetDescription().Label,
			}

			disks = append(disks, disk)
			continue
		}

		// the device is a network
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
		if virtualNetwork != nil {
			network := network{
				// Summary is the name of the network the device is attached to
				name: virtualNetwork.DeviceInfo.GetDescription().Summary,
				mac: virtualNetwork.MacAddress,
			}
			networks = append(networks, network)
		}
	}
	r.disks = &disks
	r.networks = &networks
	return nil
}

func (r *VmwareMapper) getStorageClassForDisk(disk *disk) *string {
	if r.mappings.DiskMappings != nil {
		for _, mapping := range *r.mappings.DiskMappings {
			targetName := mapping.Target.Name
			if mapping.Source.ID != nil {
				if disk.id == *mapping.Source.ID {
					if targetName != DefaultStorageClassTargetName {
						return &targetName
					}
				}
			}
			if mapping.Source.Name != nil {
				if disk.name == *mapping.Source.Name {
					if targetName != DefaultStorageClassTargetName {
						return &targetName
					}
				}
			}
		}
	}

	if r.mappings.StorageMappings != nil {
		for _, mapping := range *r.mappings.StorageMappings {
			/* compare datastore moRef */
			if disk.datastore == *mapping.Source.ID {
				return &mapping.Target.Name
			}
		}
	}
	return nil
}

// MapDataVolumes maps the VMware disks to CDI DataVolumes
func (r *VmwareMapper) MapDataVolumes(targetVMName *string) (map[string]cdiv1.DataVolume, error) {
	err := r.buildDevices()
	if err != nil {
		return nil, err
	}

	dvs := make(map[string]cdiv1.DataVolume)

	for _, disk := range *r.disks {
		dvName := buildDataVolumeName(*targetVMName, disk.name)

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
					Blank: &cdiv1.DataVolumeBlankImage{},
				},
				PVC: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: disk.capacity,
						},
					},
				},
			},
		}
		sdClass := r.getStorageClassForDisk(&disk)
		if sdClass != nil {
			dvs[dvName].Spec.PVC.StorageClassName = sdClass
		}
		r.disksByDVName[dvName] = disk
	}
	return dvs, nil
}

// MapDisk maps a disk from the VMware VM to the Kubevirt VM.
func (r *VmwareMapper) MapDisk(vmSpec *kubevirtv1.VirtualMachine, dv cdiv1.DataVolume) {
	volume := kubevirtv1.Volume{
		Name: fmt.Sprintf("dv-%v", dv.Name),
		VolumeSource: kubevirtv1.VolumeSource{
			DataVolume: &kubevirtv1.DataVolumeSource{
				Name: dv.Name,
			},
		},
	}

	disk := r.disksByDVName[dv.Name]
	kubevirtDisk := kubevirtv1.Disk{
		Name: fmt.Sprintf("dv-%v", dv.Name),
		DiskDevice: kubevirtv1.DiskDevice{
			Disk: &kubevirtv1.DiskTarget{
				Bus: disk.bus,
			},
		},
	}


	vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, volume)
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, kubevirtDisk)
}

// ResolveVMName resolves the target VM name
func (r *VmwareMapper) ResolveVMName(targetVMName *string) *string {
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

	// Map hostname
	hostname, _ := utils.NormalizeName(r.vmProperties.Guest.HostName)
	// if this is a FQDN, split off the first subdomain and use it as the hostname.
	nameParts := strings.Split(hostname, ".")
	vmSpec.Spec.Template.Spec.Hostname = nameParts[0]

	vmSpec.Spec.Template.Spec.Domain.CPU = r.mapCPUTopology()
	vmSpec.Spec.Template.Spec.Domain.Firmware = r.mapFirmware()
	reservations, err := r.mapResourceReservations()
	if err != nil {
		return nil, err
	}
	vmSpec.Spec.Template.Spec.Domain.Resources = reservations
	// Map labels like vm tags
	vmSpec.ObjectMeta.Labels = r.mapLabels(vmSpec.ObjectMeta.Labels)
	// Map annotations
	vmSpec.ObjectMeta.Annotations = r.mapAnnotations()
	running := false
	vmSpec.Spec.Running = &running
	// Map clock
	vmSpec.Spec.Template.Spec.Domain.Clock = r.mapClock(r.hostProperties)

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
	labels[LabelTag] = strings.Join(tagList, ",")
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

func (r *VmwareMapper) mapFirmware() *kubevirtv1.Firmware {
	firmwareSpec := &kubevirtv1.Firmware{}
	firmwareSpec.Bootloader = biosTypeMapping[r.vmProperties.Config.Firmware]
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
	err := r.buildDevices()
	if err != nil {
		return nil, err
	}
	var kubevirtNetworks []kubevirtv1.Network
	for _, network := range *r.networks {
		kubevirtNet := kubevirtv1.Network{}
		for _, mapping := range *r.mappings.NetworkMappings {
			if mapping.Source.Name != nil && network.name == *mapping.Source.Name {
				if *mapping.Type == networkTypePod {
					kubevirtNet.Pod = &kubevirtv1.PodNetwork{}
				} else if *mapping.Type == networkTypeMultus {
					kubevirtNet.Multus = &kubevirtv1.MultusNetwork{
						NetworkName: mapping.Target.Name,
					}
				}
			}
		}
		kubevirtNet.Name, _ = utils.NormalizeName(network.name)
		kubevirtNetworks = append(kubevirtNetworks, kubevirtNet)
	}

	return kubevirtNetworks, nil
}

func (r *VmwareMapper) mapNetworkInterfaces(networkToType map[string]string) ([]kubevirtv1.Interface, error) {
	err := r.buildDevices()
	if err != nil {
		return nil, err
	}
	var interfaces []kubevirtv1.Interface

	for _, net := range *r.networks {
		kubevirtInterface := kubevirtv1.Interface{}
		kubevirtInterface.MacAddress = net.mac
		kubevirtInterface.Name, _ = utils.NormalizeName(net.name)
		switch networkToType[kubevirtInterface.Name] {
		case networkTypeMultus:
			kubevirtInterface.Bridge = &kubevirtv1.InterfaceBridge{}
		case networkTypePod:
			kubevirtInterface.Masquerade = &kubevirtv1.InterfaceMasquerade{}
		}
		interfaces = append(interfaces, kubevirtInterface)
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

func buildDataVolumeName(targetVMName string, diskName string) string {
	dvName, _ := utils.NormalizeName(targetVMName + "-" + diskName)
	return dvName
}

func getCapacityForVirtualDisk(disk *types.VirtualDisk) (resource.Quantity, error) {
	var capacity resource.Quantity
	var capacityInBytes int64

	// which of these is populated depends on the version of vCenter
	if disk.CapacityInBytes > 0 {
		capacityInBytes = disk.CapacityInBytes
	} else {
		capacityInBytes = disk.CapacityInKB * 1024
	}
	diskSizeConverted, err := utils.FormatBytes(capacityInBytes)
	if err != nil {
		return capacity, err
	}
	capacity, err = resource.ParseQuantity(diskSizeConverted)
	if err != nil {
		return capacity, err
	}
	return capacity, nil
}
