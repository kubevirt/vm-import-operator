package guestconversion

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

var _ = Describe("GuestConversion", func() {
	volumeModeBlock := v1.PersistentVolumeBlock
	volumeModeFilesystem := v1.PersistentVolumeFilesystem

	Describe("MakeGuestConversionPodSpec", func() {

		var configMap *v1.ConfigMap
		var vmSpec *kubevirtv1.VirtualMachine
		var volumes []kubevirtv1.Volume
		var dataVolumes map[string]cdiv1.DataVolume

		BeforeEach(func() {
			configMap = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testMap",
				},
			}
			volumes = []kubevirtv1.Volume{}
			vmSpec = &kubevirtv1.VirtualMachine{
				Spec: kubevirtv1.VirtualMachineSpec{
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: volumes,
						},
					},
				},
			}

			dataVolumes = map[string]cdiv1.DataVolume{
				"dv-1": {
					ObjectMeta: metav1.ObjectMeta{Name: "dv-1"},
					Spec: cdiv1.DataVolumeSpec{
						PVC: &v1.PersistentVolumeClaimSpec{VolumeMode: &volumeModeFilesystem},
					},
				},
				"dv-2": {
					ObjectMeta: metav1.ObjectMeta{Name: "dv-2"},
					Spec: cdiv1.DataVolumeSpec{
						PVC: &v1.PersistentVolumeClaimSpec{VolumeMode: &volumeModeFilesystem},
					},
				},
				"dv-block": {
					ObjectMeta: metav1.ObjectMeta{Name: "dv-block"},
					Spec: cdiv1.DataVolumeSpec{
						PVC: &v1.PersistentVolumeClaimSpec{VolumeMode: &volumeModeBlock},
					},
				},
			}
		})

		It("should create a volume and mount for the libvirt domain config map", func() {
			pod := MakeGuestConversionPodSpec(vmSpec, dataVolumes, configMap)
			Expect(len(pod.Spec.Volumes)).To(Equal(1))
			Expect(pod.Spec.Volumes[0].Name).To(Equal(configMapVolumeName))
			Expect(pod.Spec.Volumes[0].ConfigMap).ToNot(BeNil())
			Expect(pod.Spec.Volumes[0].ConfigMap.Name).To(Equal("testMap"))
			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(1))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(configMapVolumeName))
		})

		It("should create a volume and mount for each volume that belongs to the VM", func() {
			vmSpec.Spec.Template.Spec.Volumes = []kubevirtv1.Volume{
				{
					Name: "dv-1",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-1"},
					},
				},
				{
					Name: "dv-2",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-2"},
					},
				},
				{
					Name: "dv-block",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-block"},
					},
				},
			}
			pod := MakeGuestConversionPodSpec(vmSpec, dataVolumes, configMap)
			Expect(len(pod.Spec.Volumes)).To(Equal(4))
			Expect(pod.Spec.Volumes[0].Name).To(Equal("dv-1"))
			Expect(pod.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("dv-1"))

			Expect(pod.Spec.Volumes[1].Name).To(Equal("dv-2"))
			Expect(pod.Spec.Volumes[1].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("dv-2"))

			Expect(pod.Spec.Volumes[2].Name).To(Equal("dv-block"))
			Expect(pod.Spec.Volumes[2].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("dv-block"))

			Expect(pod.Spec.Volumes[3].Name).To(Equal(configMapVolumeName))
			Expect(pod.Spec.Volumes[3].ConfigMap).ToNot(BeNil())
			Expect(pod.Spec.Volumes[3].ConfigMap.Name).To(Equal("testMap"))

			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(3))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("dv-1"))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/mnt/disks/disk0"))
			Expect(pod.Spec.Containers[0].VolumeMounts[1].Name).To(Equal("dv-2"))
			Expect(pod.Spec.Containers[0].VolumeMounts[1].MountPath).To(Equal("/mnt/disks/disk1"))
			Expect(pod.Spec.Containers[0].VolumeMounts[2].Name).To(Equal(configMapVolumeName))
			Expect(pod.Spec.Containers[0].VolumeDevices[0].Name).To(Equal("dv-block"))
			Expect(pod.Spec.Containers[0].VolumeDevices[0].DevicePath).To(Equal("/dev/block2"))
		})
	})

	Describe("MakeLibvirtDomain", func() {
		var vmSpec *kubevirtv1.VirtualMachine
		var volumes []kubevirtv1.Volume
		var dataVolumes map[string]cdiv1.DataVolume

		BeforeEach(func() {
			volumes = []kubevirtv1.Volume{
				{
					Name: "dv-1",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-1"},
					},
				},
				{
					Name: "dv-2",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-2"},
					},
				},
				{
					Name: "dv-block",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-block"},
					},
				},
			}
			vmSpec = &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "my-vm"},
				Spec: kubevirtv1.VirtualMachineSpec{
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: volumes,
							Domain: kubevirtv1.DomainSpec{
								Resources: kubevirtv1.ResourceRequirements{
									Requests: v1.ResourceList{
										v1.ResourceMemory: resource.Quantity{},
									},
									Limits:                  nil,
									OvercommitGuestOverhead: false,
								},
								CPU: &kubevirtv1.CPU{
									Sockets: 2,
									Cores:   4,
								},
								Machine: kubevirtv1.Machine{},
								Devices: kubevirtv1.Devices{},
							},
						},
					},
				},
			}
			dataVolumes = map[string]cdiv1.DataVolume{
				"dv-1": {
					ObjectMeta: metav1.ObjectMeta{Name: "dv-1"},
					Spec: cdiv1.DataVolumeSpec{
						PVC: &v1.PersistentVolumeClaimSpec{VolumeMode: &volumeModeFilesystem},
					},
				},
				"dv-2": {
					ObjectMeta: metav1.ObjectMeta{Name: "dv-2"},
					Spec: cdiv1.DataVolumeSpec{
						PVC: &v1.PersistentVolumeClaimSpec{VolumeMode: &volumeModeFilesystem},
					},
				},
				"dv-block": {
					ObjectMeta: metav1.ObjectMeta{Name: "dv-block"},
					Spec: cdiv1.DataVolumeSpec{
						PVC: &v1.PersistentVolumeClaimSpec{VolumeMode: &volumeModeBlock},
					},
				},
			}
		})

		It("should create a DomainDisk for each volume on the VM", func() {
			domain := MakeLibvirtDomain(vmSpec, dataVolumes)
			Expect(len(domain.Devices.Disks)).To(Equal(3))
			Expect(domain.Devices.Disks[0].Source.File.File).To(Equal("/mnt/disks/disk0/disk.img"))
			Expect(domain.Devices.Disks[0].Target.Dev).To(Equal("hda"))
			Expect(domain.Devices.Disks[1].Source.File.File).To(Equal("/mnt/disks/disk1/disk.img"))
			Expect(domain.Devices.Disks[1].Target.Dev).To(Equal("hdb"))
			Expect(domain.Devices.Disks[2].Source.Block.Dev).To(Equal("/dev/block2"))
			Expect(domain.Devices.Disks[2].Target.Dev).To(Equal("hdc"))
		})
	})
})
