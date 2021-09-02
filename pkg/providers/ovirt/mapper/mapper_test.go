package mapper_test

import (
	"fmt"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

const memoryGI = 1024 * 1024 * 1024
const maxMemoryGI = 4 * 1024 * 1024 * 1024

var (
	targetVMName       = "myvm"
	expectedDVName     = "d7bc458c04f4863b9e517c9b1661fbd34c02dc02"
	filesystemOverhead = cdiv1.FilesystemOverhead{
		Global: "0.0",
	}
)

var (
	findOs   func(vm *ovirtsdk.Vm) (string, error)
	osFinder = mockOsFinder{}
)

var _true bool = true
var _false bool = false

var _ = Describe("Test mapping virtual machine bios type", func() {
	BeforeEach(func() {
		findOs = func(vm *ovirtsdk.Vm) (string, error) {
			return "linux", nil
		}
	})

	table.DescribeTable("to smm feature ", func(biostype ovirtsdk.BiosType, smm *bool) {
		vm := createVMGeneric(ovirtsdk.VMAFFINITY_USER_MIGRATABLE, true, biostype, ovirtsdk.DISKINTERFACE_VIRTIO)

		mappings := createMappings()
		credentials := mapper.DataVolumeCredentials{
			URL:           "any-url",
			SecretName:    "secret-name",
			ConfigMapName: "config-map",
		}
		namespace := "the-namespace"
		mapper := mapper.NewOvirtMapper(vm, &mappings, credentials, namespace, &osFinder)
		vmSpec, _ := mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		Expect(vmSpec.Spec.Template.Spec.Domain.Features).ToNot(BeNil())
		if *smm {
			Expect(vmSpec.Spec.Template.Spec.Domain.Features.SMM.Enabled).To(Equal(smm))
		} else {
			Expect(vmSpec.Spec.Template.Spec.Domain.Features.SMM).To(BeNil())
		}

	},
		table.Entry("q35_ovmf", ovirtsdk.BIOSTYPE_Q35_OVMF, &_true),
		table.Entry("q35_secure_boot", ovirtsdk.BIOSTYPE_Q35_SECURE_BOOT, &_false),
		table.Entry("q35_sea_bios", ovirtsdk.BIOSTYPE_Q35_SEA_BIOS, &_false),
	)
})

var _ = Describe("Test mapping virtual machine attributes", func() {
	var (
		vm       *ovirtsdk.Vm
		vmSpec   *kubevirtv1.VirtualMachine
		mappings v2vv1.OvirtMappings
	)

	BeforeEach(func() {
		vm = createVM()
		mappings = createMappings()
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)

		findOs = func(vm *ovirtsdk.Vm) (string, error) {
			return "linux", nil
		}

		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})
	})

	It("should map name", func() {
		Expect(vmSpec.Name).To(Equal(vm.MustName()))
	})

	It("should map CPU topology", func() {
		vmTopology := vm.MustCpu().MustTopology()
		vmSpecCPU := vmSpec.Spec.Template.Spec.Domain.CPU
		Expect(vmSpecCPU.Cores).To(Equal(uint32(vmTopology.MustCores())))
		Expect(vmSpecCPU.Sockets).To(Equal(uint32(vmTopology.MustSockets())))
		Expect(vmSpecCPU.Threads).To(Equal(uint32(vmTopology.MustThreads())))
	})

	It("should not override machine type", func() {
		vm = createVM()
		vm.SetCustomEmulatedMachine("pc-i440fx-rhel7.6.0")

		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		Expect(vmSpec.Spec.Template.Spec.Domain.Machine.Type).To(Equal("q35"))
	})

	It("should map CPU pinning", func() {
		vm = createVM()
		vm.MustCpu().SetCpuTune(
			ovirtsdk.NewCpuTuneBuilder().
				VcpuPinsOfAny(
					ovirtsdk.NewVcpuPinBuilder().CpuSet("0").Vcpu(0).MustBuild()).
				MustBuild())
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		vmSpecCPU := vmSpec.Spec.Template.Spec.Domain.CPU

		Expect(vmSpecCPU.DedicatedCPUPlacement).To(BeTrue())
		Expect(vmSpec.Spec.Template.Spec.Domain.Resources.Requests.Memory().Value()).To(BeEquivalentTo(maxMemoryGI))
	})

	It("should normalize hostname", func() {
		fqdn := "rhev-orange-03.rdu2.scalelab.redhat.com"
		norm, _ := utils.NormalizeLabel(fqdn)
		vm = createVM()
		vm.SetFqdn(fqdn)

		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		Expect(vmSpec.Spec.Template.Spec.Hostname).To(Equal(norm))
	})

	It("should map BIOS", func() {
		bootloader := vmSpec.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS
		Expect(bootloader).ToNot(BeNil())
	})

	It("should map HA", func() {
		Expect(*vmSpec.Spec.Running).To(Equal(vm.MustHighAvailability().MustEnabled()))
	})

	It("should map Memory", func() {
		// guest memory
		guestMemory, _ := vmSpec.Spec.Template.Spec.Domain.Resources.Requests.Memory().AsInt64()
		Expect(guestMemory).To(Equal(vm.MustMemory()))

		// huge page
		hugePageSize := vmSpec.Spec.Template.Spec.Domain.Memory.Hugepages.PageSize
		mappedHugePageSize := vm.MustCustomProperties().Slice()[0].MustValue()
		Expect(hugePageSize).To(Equal(fmt.Sprintf("%sMi", mappedHugePageSize)))
	})

	It("should map graphics consoles for non-windows OS", func() {
		devices := vmSpec.Spec.Template.Spec.Domain.Devices

		graphicsEnabled := devices.AutoattachGraphicsDevice
		Expect(*graphicsEnabled).To(Equal(len(vm.MustGraphicsConsoles().Slice()) > 0))

		Expect(devices.Inputs).To(HaveLen(1))
		inputDevice := devices.Inputs[0]
		Expect(inputDevice.Type).To(BeEquivalentTo("tablet"))
		Expect(inputDevice.Name).To(BeEquivalentTo("tablet"))

		Expect(inputDevice.Bus).To(BeEquivalentTo("virtio"))

	})

	It("should map graphics consoles for windows OS", func() {
		findOs = func(vm *ovirtsdk.Vm) (string, error) {
			return "Win2k19", nil
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ := mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		devices := vmSpec.Spec.Template.Spec.Domain.Devices

		graphicsEnabled := devices.AutoattachGraphicsDevice
		Expect(*graphicsEnabled).To(Equal(len(vm.MustGraphicsConsoles().Slice()) > 0))

		Expect(devices.Inputs).To(HaveLen(1))
		inputDevice := devices.Inputs[0]
		Expect(inputDevice.Type).To(BeEquivalentTo("tablet"))
		Expect(inputDevice.Name).To(BeEquivalentTo("tablet"))

		Expect(inputDevice.Bus).To(BeEquivalentTo("usb"))

	})

	It("should create UTC clock", func() {
		clock := vmSpec.Spec.Template.Spec.Domain.Clock
		Expect(clock.Timezone).To(BeNil())
		Expect(clock.UTC).NotTo(BeNil())
		Expect(clock.UTC.OffsetSeconds).NotTo(BeNil())
		// +01:00
		Expect(*clock.UTC.OffsetSeconds).To(BeEquivalentTo(3600))
	})

	It("should create UTC clock without offset", func() {
		vm = createVM()
		vm.SetTimeZone(ovirtsdk.NewTimeZoneBuilder().
			Name("Etc/GMT").MustBuild())
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		clock := vmSpec.Spec.Template.Spec.Domain.Clock
		Expect(clock.Timezone).To(BeNil())
		Expect(clock.UTC).NotTo(BeNil())
		Expect(clock.UTC.OffsetSeconds).To(BeNil())
	})

	It("should handle cluster default bios type", func() {
		vm = createVM()
		vm.SetBios(
			ovirtsdk.NewBiosBuilder().
				Type(ovirtsdk.BIOSTYPE_CLUSTER_DEFAULT).MustBuild())
		vm.SetCluster(
			ovirtsdk.NewClusterBuilder().BiosType(ovirtsdk.BIOSTYPE_Q35_SEA_BIOS).MustBuild())

		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		Expect(vmSpec.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS).To(Equal(&kubevirtv1.BIOS{}))
	})

	It("should enable secure boot", func() {
		_true := true
		vm = createVM()
		vm.SetBios(
			ovirtsdk.NewBiosBuilder().
				Type(ovirtsdk.BIOSTYPE_CLUSTER_DEFAULT).MustBuild())
		vm.SetCluster(
			ovirtsdk.NewClusterBuilder().BiosType(ovirtsdk.BIOSTYPE_Q35_OVMF).MustBuild())

		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		Expect(vmSpec.Spec.Template.Spec.Domain.Features.SMM.Enabled).To(Equal(&_true))
	})

	It("should create UTC clock without offset for offset parsing problem", func() {
		vm = createVM()
		vm.SetTimeZone(ovirtsdk.NewTimeZoneBuilder().
			UtcOffset("illegal").MustBuild())
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		clock := vmSpec.Spec.Template.Spec.Domain.Clock
		Expect(clock.Timezone).To(BeNil())
		Expect(clock.UTC).NotTo(BeNil())
		Expect(clock.UTC.OffsetSeconds).To(BeNil())
	})

	It("should create UTC clock when no clock in source VM", func() {
		vm = createVM()
		vm.SetTimeZone(nil)
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		clock := vmSpec.Spec.Template.Spec.Domain.Clock
		Expect(clock.Timezone).To(BeNil())
		Expect(clock.UTC).NotTo(BeNil())
		Expect(clock.UTC.OffsetSeconds).To(BeNil())
	})

	It("should map placement policy", func() {
		evictionStrategy := vmSpec.Spec.Template.Spec.EvictionStrategy
		Expect(string(*evictionStrategy)).ToNot(BeNil())
	})

	It("should map annotations", func() {
		annotations := vmSpec.ObjectMeta.Annotations
		Expect(annotations[mapper.AnnotationComment]).To(Equal(vm.MustComment()))
		Expect(annotations[mapper.AnnotationSso]).To(Equal(string(vm.MustSso().MustMethods().Slice()[0].MustId())))
	})

	It("should map labels", func() {
		labels := vmSpec.ObjectMeta.Labels
		Expect(labels[mapper.LabelOrigin]).To(Equal(vm.MustOrigin()))
		Expect(labels[mapper.LabelInstanceType]).To(Equal(vm.MustInstanceType().MustName()))
	})

	It("should map nics", func() {
		interfaces := vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces
		networks := vmSpec.Spec.Template.Spec.Networks
		networkMapping := *mappings.NetworkMappings
		nic1 := vm.MustNics().Slice()[0]
		nic2 := vm.MustNics().Slice()[1]

		Expect(interfaces[0].Name).To(Equal(nic1.MustName()))
		Expect(interfaces[0].Model).To(Equal(string(nic1.MustInterface())))
		// It's a multus network
		Expect(interfaces[0].Bridge).To(Not(BeNil()))
		Expect(interfaces[0].Masquerade).To(BeNil())
		Expect(networks[0].Name).To(Equal(nic1.MustName()))
		Expect(networks[0].Multus.NetworkName).To(Equal(networkMapping[0].Target.Name))

		Expect(interfaces[1].Name).To(Equal(nic2.MustName()))
		Expect(interfaces[1].Model).To(Equal(string(nic2.MustInterface())))
		// It's a pod network
		Expect(interfaces[1].Masquerade).To(Not(BeNil()))
		Expect(interfaces[1].Bridge).To(BeNil())
		Expect(networks[1].Name).To(Equal(nic2.MustName()))
		Expect(networks[1].Pod).To(Not(BeNil()))
	})

	It("should map sriov nic", func() {
		vm = createVM()
		slice := new(ovirtsdk.NicSlice)
		nics := []*ovirtsdk.Nic{
			ovirtsdk.NewNicBuilder().
				Name("nic1").
				Interface("pci_passthrough").
				VnicProfile(
					ovirtsdk.NewVnicProfileBuilder().
						Name("profile1").
						PassThrough(
							ovirtsdk.NewVnicPassThroughBuilder().Mode(
								ovirtsdk.VNICPASSTHROUGHMODE_ENABLED).
								MustBuild()).
						Network(
							ovirtsdk.NewNetworkBuilder().
								Name("network1").MustBuild()).
						MustBuild()).
				MustBuild(),
		}
		slice.SetSlice(nics)
		vm.SetNics(slice)
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		vmSpec, _ = mapper.MapVM(&targetVMName, &kubevirtv1.VirtualMachine{})

		interfaces := vmSpec.Spec.Template.Spec.Domain.Devices.Interfaces
		networks := vmSpec.Spec.Template.Spec.Networks
		networkMapping := *mappings.NetworkMappings

		Expect(interfaces[0].Name).To(Equal(nics[0].MustName()))
		Expect(interfaces[0].SRIOV).To(Not(BeNil()))
		Expect(interfaces[0].Bridge).To(BeNil())
		Expect(interfaces[0].Masquerade).To(BeNil())

		Expect(networks[0].Multus.NetworkName).To(Equal(vmSpec.Namespace + "/" + networkMapping[0].Target.Name))
	})
})

var _ = Describe("Test pvc accessmodes", func() {
	table.DescribeTable("should be : ", func(affinity ovirtsdk.VmAffinity, readonly bool, accessMode corev1.PersistentVolumeAccessMode) {
		vm := createVMGeneric(affinity, readonly, ovirtsdk.BIOSTYPE_Q35_SEA_BIOS, ovirtsdk.DISKINTERFACE_VIRTIO)

		mappings := createMappings()
		credentials := mapper.DataVolumeCredentials{
			URL:           "any-url",
			SecretName:    "secret-name",
			ConfigMapName: "config-map",
		}
		namespace := "the-namespace"
		mapper := mapper.NewOvirtMapper(vm, &mappings, credentials, namespace, &osFinder)
		daName := expectedDVName
		dvs, _ := mapper.MapDataVolumes(&targetVMName, filesystemOverhead)

		Expect(dvs).To(HaveLen(1))
		Expect(dvs).To(HaveKey(daName))

		dv := dvs[daName]
		Expect(dv.Namespace).To(Equal(namespace))
		Expect(dv.Name).To(Equal(daName))
		Expect(dv.Spec.PVC.AccessModes).To(HaveLen(1))
		Expect(dv.Spec.PVC.AccessModes).To(ContainElement(accessMode))
	},
		table.Entry("readwriteonce for pinned", ovirtsdk.VMAFFINITY_PINNED, false, corev1.ReadWriteOnce),
		table.Entry("readwriteonce for user migratable", ovirtsdk.VMAFFINITY_USER_MIGRATABLE, false, corev1.ReadWriteOnce),
		table.Entry("readwritemany for migratable", ovirtsdk.VMAFFINITY_MIGRATABLE, false, corev1.ReadWriteMany),
		table.Entry("readwritemany for migratable", ovirtsdk.VMAFFINITY_USER_MIGRATABLE, true, corev1.ReadOnlyMany),
	)
})

var _ = Describe("Test mapping disks", func() {
	var (
		vm                 *ovirtsdk.Vm
		filesystemOverhead = cdiv1.FilesystemOverhead{
			Global: "0.0",
		}
	)

	BeforeEach(func() {
		vm = createVM()
	})

	It("should map disk", func() {
		mappings := createMappings()
		credentials := mapper.DataVolumeCredentials{
			URL:           "any-url",
			SecretName:    "secret-name",
			ConfigMapName: "config-map",
		}
		namespace := "the-namespace"
		mapper := mapper.NewOvirtMapper(vm, &mappings, credentials, namespace, &osFinder)
		daName := expectedDVName
		dvs, _ := mapper.MapDataVolumes(&targetVMName, filesystemOverhead)

		Expect(dvs).To(HaveLen(1))
		Expect(dvs).To(HaveKey(daName))

		dv := dvs[daName]
		Expect(dv.Namespace).To(Equal(namespace))
		Expect(dv.Name).To(Equal(daName))

		Expect(dv.Spec.Source.Imageio).To(Not(BeNil()))
		imageio := *dv.Spec.Source.Imageio
		Expect(imageio.URL).To(Equal(credentials.URL))
		Expect(imageio.CertConfigMap).To(Equal(credentials.ConfigMapName))
		Expect(imageio.SecretRef).To(Equal(credentials.SecretName))
		Expect(imageio.DiskID).To(Equal("disk-ID"))

		Expect(dv.Spec.PVC.AccessModes).To(ContainElement(corev1.ReadWriteMany))
		Expect(dv.Spec.PVC.AccessModes).To(HaveLen(1))
		Expect(dv.Spec.PVC.Resources.Requests).To(HaveKey(corev1.ResourceStorage))
		storageResource := dv.Spec.PVC.Resources.Requests[corev1.ResourceStorage]
		Expect(storageResource.Value()).To(BeEquivalentTo(1073741824))

		Expect(dv.Spec.PVC.StorageClassName).To(Not(BeNil()))
		Expect(*dv.Spec.PVC.StorageClassName).To(Equal("storageclassname"))
	})

	It("should map disk with default overhead", func() {
		mappings := createMappings()
		credentials := mapper.DataVolumeCredentials{
			URL:           "any-url",
			SecretName:    "secret-name",
			ConfigMapName: "config-map",
		}
		namespace := "the-namespace"
		mapper := mapper.NewOvirtMapper(vm, &mappings, credentials, namespace, &osFinder)
		daName := expectedDVName

		// request 100% overhead, resulting in a disk of twice the size.
		overhead := cdiv1.FilesystemOverhead{
			Global: "0.055",
		}
		dvs, _ := mapper.MapDataVolumes(&targetVMName, overhead)

		Expect(dvs).To(HaveLen(1))
		Expect(dvs).To(HaveKey(daName))

		dv := dvs[expectedDVName]

		Expect(dv.Spec.PVC.Resources.Requests).To(HaveKey(corev1.ResourceStorage))
		storageResource := dv.Spec.PVC.Resources.Requests[corev1.ResourceStorage]
		Expect(storageResource.Value()).To(BeEquivalentTo(1136656384))

		Expect(dv.Spec.PVC.StorageClassName).To(Not(BeNil()))
		Expect(*dv.Spec.PVC.StorageClassName).To(Equal("storageclassname"))
	})

	It("should map disk with storage class specific overhead", func() {
		mappings := createMappings()
		credentials := mapper.DataVolumeCredentials{
			URL:           "any-url",
			SecretName:    "secret-name",
			ConfigMapName: "config-map",
		}
		namespace := "the-namespace"
		mapper := mapper.NewOvirtMapper(vm, &mappings, credentials, namespace, &osFinder)
		daName := expectedDVName
		scName := "storageclassname"
		// request 100% overhead for the storage class, resulting in a disk of twice the size.
		overhead := cdiv1.FilesystemOverhead{
			Global: "0.0",
			StorageClass: map[string]cdiv1.Percent{
				scName: "0.055",
			},
		}
		dvs, _ := mapper.MapDataVolumes(&targetVMName, overhead)

		Expect(dvs).To(HaveLen(1))
		Expect(dvs).To(HaveKey(daName))

		dv := dvs[expectedDVName]

		Expect(dv.Spec.PVC.Resources.Requests).To(HaveKey(corev1.ResourceStorage))
		storageResource := dv.Spec.PVC.Resources.Requests[corev1.ResourceStorage]
		Expect(storageResource.Value()).To(BeEquivalentTo(1136656384))

		Expect(dv.Spec.PVC.StorageClassName).To(Not(BeNil()))
		Expect(*dv.Spec.PVC.StorageClassName).To(Equal(scName))
	})

	It("should map disk storage class from disk", func() {
		diskID := "disk-ID"
		targetStorageClass := "storageclassname"
		fsMode := corev1.PersistentVolumeFilesystem
		disks := []v2vv1.StorageResourceMappingItem{
			{
				Source: v2vv1.Source{
					ID: &diskID,
				},
				Target: v2vv1.ObjectIdentifier{
					Name: targetStorageClass,
				},
				VolumeMode: &fsMode,
			},
		}
		mappings := v2vv1.OvirtMappings{
			DiskMappings:    &disks,
			StorageMappings: &[]v2vv1.StorageResourceMappingItem{},
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)

		dvs, _ := mapper.MapDataVolumes(&targetVMName, filesystemOverhead)

		Expect(dvs).To(HaveLen(1))
		Expect(dvs[expectedDVName].Spec.PVC.StorageClassName).To(Not(BeNil()))
		Expect(*dvs[expectedDVName].Spec.PVC.StorageClassName).To(Equal(targetStorageClass))
		Expect(*dvs[expectedDVName].Spec.PVC.VolumeMode).To(Equal(corev1.PersistentVolumeFilesystem))
	})

	It("should map empty disk storage class to nil", func() {
		diskID := "disk-ID"
		targetStorageClass := ""
		blockMode := corev1.PersistentVolumeBlock
		disks := []v2vv1.StorageResourceMappingItem{
			{
				Source: v2vv1.Source{
					ID: &diskID,
				},
				Target: v2vv1.ObjectIdentifier{
					Name: targetStorageClass,
				},
				VolumeMode: &blockMode,
			},
		}
		mappings := v2vv1.OvirtMappings{
			DiskMappings:    &disks,
			StorageMappings: &[]v2vv1.StorageResourceMappingItem{},
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)

		dvs, err := mapper.MapDataVolumes(&targetVMName, filesystemOverhead)

		Expect(err).To(BeNil())
		Expect(dvs).To(HaveLen(1))
		Expect(dvs[expectedDVName].Spec.PVC.StorageClassName).To(BeNil())
		Expect(*dvs[expectedDVName].Spec.PVC.VolumeMode).To(Equal(corev1.PersistentVolumeBlock))
	})
	It("should map empty storage domain storage class to nil", func() {
		storageDomainName := "mystoragedomain"
		targetStorageClass := ""
		domains := []v2vv1.StorageResourceMappingItem{
			{
				Source: v2vv1.Source{
					Name: &storageDomainName,
				},
				Target: v2vv1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}
		mappings := v2vv1.OvirtMappings{
			DiskMappings:    &[]v2vv1.StorageResourceMappingItem{},
			StorageMappings: &domains,
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)

		dvs, err := mapper.MapDataVolumes(&targetVMName, filesystemOverhead)

		Expect(err).To(BeNil())
		Expect(dvs).To(HaveLen(1))
		Expect(dvs[expectedDVName].Spec.PVC.StorageClassName).To(BeNil())
	})
	It("should map missing mapping to nil storage class", func() {
		mappings := v2vv1.OvirtMappings{
			DiskMappings:    &[]v2vv1.StorageResourceMappingItem{},
			StorageMappings: &[]v2vv1.StorageResourceMappingItem{},
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)

		dvs, err := mapper.MapDataVolumes(&targetVMName, filesystemOverhead)

		Expect(err).To(BeNil())
		Expect(dvs).To(HaveLen(1))
		Expect(dvs[expectedDVName].Spec.PVC.StorageClassName).To(BeNil())
	})

	table.DescribeTable("should map disk bus type: ", func(diskInterface ovirtsdk.DiskInterface) {
		vm := createVMGeneric(ovirtsdk.VMAFFINITY_MIGRATABLE, false, ovirtsdk.BIOSTYPE_Q35_SEA_BIOS, diskInterface)
		vmSpec := &kubevirtv1.VirtualMachine{
			ObjectMeta: v1.ObjectMeta{
				Name: targetVMName,
			},
		}
		vmSpec.Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{}

		diskID := "disk-ID"
		targetStorageClass := "storageclassname"
		disks := []v2vv1.StorageResourceMappingItem{
			{
				Source: v2vv1.Source{
					ID: &diskID,
				},
				Target: v2vv1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}
		mappings := v2vv1.OvirtMappings{
			DiskMappings:    &disks,
			StorageMappings: &[]v2vv1.StorageResourceMappingItem{},
		}

		mapper_ := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "", &osFinder)
		dvs, _ := mapper_.MapDataVolumes(&targetVMName, filesystemOverhead)
		mapper_.MapDisk(vmSpec, dvs[expectedDVName])
		Expect(vmSpec.Spec.Template.Spec.Domain.Devices.Disks[0].Disk.Bus).To(Equal(mapper.DiskInterfaceModelMapping[string(diskInterface)]))
	},
		table.Entry("virtio to virtio", ovirtsdk.DISKINTERFACE_VIRTIO),
		table.Entry("sata to sata", ovirtsdk.DISKINTERFACE_SATA),
		table.Entry("virtio_scsi to scsi", ovirtsdk.DISKINTERFACE_VIRTIO_SCSI),
	)
})

func createVM() *ovirtsdk.Vm {
	return createVMGeneric(ovirtsdk.VMAFFINITY_MIGRATABLE, false, ovirtsdk.BIOSTYPE_Q35_SEA_BIOS, ovirtsdk.DISKINTERFACE_VIRTIO)
}

func createVMGeneric(affinity ovirtsdk.VmAffinity, readonly bool, biostype ovirtsdk.BiosType, interfaceType ovirtsdk.DiskInterface) *ovirtsdk.Vm {
	return ovirtsdk.NewVmBuilder().
		Name("myvm").
		Bios(
			ovirtsdk.NewBiosBuilder().
				Type(biostype).MustBuild()).
		Cpu(
			ovirtsdk.NewCpuBuilder().
				Topology(
					ovirtsdk.NewCpuTopologyBuilder().
						Cores(1).
						Sockets(2).
						Threads(4).
						MustBuild()).
				Architecture(ovirtsdk.ARCHITECTURE_X86_64).
				MustBuild()).
		HighAvailability(
			ovirtsdk.NewHighAvailabilityBuilder().
				Enabled(true).
				MustBuild()).
		Memory(memoryGI).
		MemoryPolicy(
			ovirtsdk.NewMemoryPolicyBuilder().
				Max(maxMemoryGI).MustBuild()).
		GraphicsConsolesOfAny(
			ovirtsdk.NewGraphicsConsoleBuilder().
				Name("testConsole").MustBuild()).
		TimeZone(
			ovirtsdk.NewTimeZoneBuilder().
				UtcOffset("+01:00").
				MustBuild()).
		PlacementPolicy(
			ovirtsdk.NewVmPlacementPolicyBuilder().
				Affinity(affinity).MustBuild()).
		Origin("ovirt").
		Comment("vmcomment").
		InstanceType(
			ovirtsdk.NewInstanceTypeBuilder().
				Name("server").MustBuild()).
		TagsOfAny(
			ovirtsdk.NewTagBuilder().
				Name("mytag").MustBuild()).
		CustomPropertiesOfAny(
			ovirtsdk.NewCustomPropertyBuilder().
				Name("hugepages").
				Value("2048").MustBuild()).
		Sso(
			ovirtsdk.NewSsoBuilder().
				MethodsOfAny(
					ovirtsdk.NewMethodBuilder().
						Id(ovirtsdk.SSOMETHOD_GUEST_AGENT).
						MustBuild()).
				MustBuild()).
		NicsOfAny(
			ovirtsdk.NewNicBuilder().
				Name("nic1").
				Interface("virtio").
				VnicProfile(
					ovirtsdk.NewVnicProfileBuilder().
						Name("profile1").
						Network(
							ovirtsdk.NewNetworkBuilder().
								Name("network1").MustBuild()).
						MustBuild()).
				MustBuild(),
			ovirtsdk.NewNicBuilder().
				Name("nic2").
				Interface("virtio").
				VnicProfile(
					ovirtsdk.NewVnicProfileBuilder().
						Name("profile2").
						Network(
							ovirtsdk.NewNetworkBuilder().
								Name("network2").MustBuild()).
						MustBuild()).
				MustBuild()).
		DiskAttachmentsOfAny(
			ovirtsdk.NewDiskAttachmentBuilder().
				Id("123").
				ReadOnly(readonly).
				Interface(interfaceType).
				Disk(
					ovirtsdk.NewDiskBuilder().
						Id("disk-ID").
						Name("mydisk").
						Bootable(true).
						ProvisionedSize(memoryGI).
						StorageDomain(
							ovirtsdk.NewStorageDomainBuilder().
								Name("mystoragedomain").MustBuild()).
						MustBuild()).MustBuild()).
		MustBuild()
}

func createMappings() v2vv1.OvirtMappings {
	// network mappings
	var networks []v2vv1.NetworkResourceMappingItem
	multusNetwork := "multus"
	podNetwork := "pod"
	multusNetworkName := "network1/profile1"
	podNetworkName := "network2/profile2"
	networks = append(networks,
		v2vv1.NetworkResourceMappingItem{
			Source: v2vv1.Source{
				Name: &multusNetworkName,
			},
			Target: v2vv1.ObjectIdentifier{
				Name: "net-attach-def",
			},
			Type: &multusNetwork,
		},
		v2vv1.NetworkResourceMappingItem{
			Source: v2vv1.Source{
				Name: &podNetworkName,
			},
			Type: &podNetwork,
		})

	// storage mappings
	var storages []v2vv1.StorageResourceMappingItem
	storageName := "mystoragedomain"
	storages = append(storages, v2vv1.StorageResourceMappingItem{
		Source: v2vv1.Source{
			Name: &storageName,
		},
		Target: v2vv1.ObjectIdentifier{
			Name: "storageclassname",
		},
	})

	return v2vv1.OvirtMappings{
		NetworkMappings: &networks,
		StorageMappings: &storages,
		DiskMappings:    &[]v2vv1.StorageResourceMappingItem{},
	}
}

type mockOsFinder struct{}

func (o *mockOsFinder) FindOperatingSystem(vm *ovirtsdk.Vm) (string, error) {
	return findOs(vm)
}
