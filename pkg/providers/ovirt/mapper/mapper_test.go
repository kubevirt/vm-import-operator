package mapper_test

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

const memoryGI = 1024 * 1024 * 1024

var targetVMName = "myvm"

var _ = Describe("Test mapping virtual machine attributes", func() {
	var (
		vm       *ovirtsdk.Vm
		vmSpec   *kubevirtv1.VirtualMachine
		mappings v2vv1alpha1.OvirtMappings
	)

	BeforeEach(func() {
		vm = createVM()
		mappings = createMappings()
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "")
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

		// max memory
		memLimit, _ := vmSpec.Spec.Template.Spec.Domain.Resources.Limits.Memory().AsInt64()
		memMax := vm.MustMemoryPolicy().MustMax()
		Expect(memLimit).To(Equal(memMax))
	})

	It("should map graphics consoles", func() {
		graphicsEnabled := vmSpec.Spec.Template.Spec.Domain.Devices.AutoattachGraphicsDevice
		Expect(*graphicsEnabled).To(Equal(len(vm.MustGraphicsConsoles().Slice()) > 0))
	})

	It("should create UTC clock", func() {
		clock := vmSpec.Spec.Template.Spec.Domain.Clock
		Expect(clock.Timezone).To(BeNil())
		Expect(clock.UTC).To(Not(BeNil()))
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
})

var _ = Describe("Test mapping disks", func() {
	var (
		vm *ovirtsdk.Vm
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
		mapper := mapper.NewOvirtMapper(vm, &mappings, credentials, namespace)
		daName := "123"
		dvs, _ := mapper.MapDisks()

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

		Expect(dv.Spec.PVC.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
		Expect(dv.Spec.PVC.AccessModes).To(HaveLen(1))
		Expect(dv.Spec.PVC.Resources.Requests).To(HaveKey(corev1.ResourceStorage))
		storageResource := dv.Spec.PVC.Resources.Requests[corev1.ResourceStorage]
		Expect(storageResource.Format).To(Equal(resource.BinarySI))
		Expect(storageResource.Value()).To(BeEquivalentTo(memoryGI))

		Expect(dv.Spec.PVC.StorageClassName).To(Not(BeNil()))
		Expect(*dv.Spec.PVC.StorageClassName).To(Equal("storageclassname"))
	})

	It("should map disk storage class from disk", func() {
		diskID := "disk-ID"
		targetStorageClass := "storageclassname"
		disks := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					ID: &diskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}
		mappings := v2vv1alpha1.OvirtMappings{
			DiskMappings:    &disks,
			StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{},
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "")

		dvs, _ := mapper.MapDisks()

		Expect(dvs).To(HaveLen(1))
		Expect(dvs["123"].Spec.PVC.StorageClassName).To(Not(BeNil()))
		Expect(*dvs["123"].Spec.PVC.StorageClassName).To(Equal(targetStorageClass))
	})

	It("should map empty disk storage class to nil", func() {
		diskID := "disk-ID"
		targetStorageClass := ""
		disks := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					ID: &diskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}
		mappings := v2vv1alpha1.OvirtMappings{
			DiskMappings:    &disks,
			StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{},
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "")

		dvs, err := mapper.MapDisks()

		Expect(err).To(BeNil())
		Expect(dvs).To(HaveLen(1))
		Expect(dvs["123"].Spec.PVC.StorageClassName).To(BeNil())
	})
	It("should map empty storage domain storage class to nil", func() {
		storageDomainName := "mystoragedomain"
		targetStorageClass := ""
		domains := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: &storageDomainName,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}
		mappings := v2vv1alpha1.OvirtMappings{
			DiskMappings:    &[]v2vv1alpha1.ResourceMappingItem{},
			StorageMappings: &domains,
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "")

		dvs, err := mapper.MapDisks()

		Expect(err).To(BeNil())
		Expect(dvs).To(HaveLen(1))
		Expect(dvs["123"].Spec.PVC.StorageClassName).To(BeNil())
	})
	It("should map missing mapping to nil storage class", func() {
		mappings := v2vv1alpha1.OvirtMappings{
			DiskMappings:    &[]v2vv1alpha1.ResourceMappingItem{},
			StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{},
		}
		mapper := mapper.NewOvirtMapper(vm, &mappings, mapper.DataVolumeCredentials{}, "")

		dvs, err := mapper.MapDisks()

		Expect(err).To(BeNil())
		Expect(dvs).To(HaveLen(1))
		Expect(dvs["123"].Spec.PVC.StorageClassName).To(BeNil())
	})

})

func createVM() *ovirtsdk.Vm {
	return ovirtsdk.NewVmBuilder().
		Name("myvm").
		Bios(
			ovirtsdk.NewBiosBuilder().
				Type(ovirtsdk.BIOSTYPE_Q35_SEA_BIOS).MustBuild()).
		Cpu(
			ovirtsdk.NewCpuBuilder().
				Topology(
					ovirtsdk.NewCpuTopologyBuilder().
						Cores(1).
						Sockets(2).
						Threads(4).
						MustBuild()).
				MustBuild()).
		HighAvailability(
			ovirtsdk.NewHighAvailabilityBuilder().
				Enabled(true).
				MustBuild()).
		Memory(memoryGI).
		MemoryPolicy(
			ovirtsdk.NewMemoryPolicyBuilder().
				Max(memoryGI).MustBuild()).
		GraphicsConsolesOfAny(
			ovirtsdk.NewGraphicsConsoleBuilder().
				Name("testConsole").MustBuild()).
		TimeZone(
			ovirtsdk.NewTimeZoneBuilder().
				Name(">Etc/GMT").MustBuild()).
		PlacementPolicy(
			ovirtsdk.NewVmPlacementPolicyBuilder().
				Affinity(ovirtsdk.VMAFFINITY_MIGRATABLE).MustBuild()).
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
						Network(
							ovirtsdk.NewNetworkBuilder().
								Name("network2").MustBuild()).
						MustBuild()).
				MustBuild()).
		DiskAttachmentsOfAny(
			ovirtsdk.NewDiskAttachmentBuilder().
				Id("123").
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

func createMappings() v2vv1alpha1.OvirtMappings {
	// network mappings
	var networks []v2vv1alpha1.ResourceMappingItem
	multusNetwork := "multus"
	podNetwork := "pod"
	multusNetworkName := "network1"
	podNetworkName := "network2"
	networks = append(networks,
		v2vv1alpha1.ResourceMappingItem{
			Source: v2vv1alpha1.Source{
				Name: &multusNetworkName,
			},
			Target: v2vv1alpha1.ObjectIdentifier{
				Name: "net-attach-def",
			},
			Type: &multusNetwork,
		},
		v2vv1alpha1.ResourceMappingItem{
			Source: v2vv1alpha1.Source{
				Name: &podNetworkName,
			},
			Type: &podNetwork,
		})

	// storage mappings
	var storages []v2vv1alpha1.ResourceMappingItem
	storageName := "mystoragedomain"
	storages = append(storages, v2vv1alpha1.ResourceMappingItem{
		Source: v2vv1alpha1.Source{
			Name: &storageName,
		},
		Target: v2vv1alpha1.ObjectIdentifier{
			Name: "storageclassname",
		},
	})

	return v2vv1alpha1.OvirtMappings{
		NetworkMappings: &networks,
		StorageMappings: &storages,
		DiskMappings:    &[]v2vv1alpha1.ResourceMappingItem{},
	}
}
