package guestconversion

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("GuestConversion", func() {

	Describe("MakeGuestConversionJobSpec", func() {

		var configMap *v1.ConfigMap
		var vmSpec *kubevirtv1.VirtualMachine
		var volumes []kubevirtv1.Volume

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
		})

		It("should create a volume and mount for the libvirt domain config map", func() {
			job := MakeGuestConversionJobSpec(vmSpec, configMap, "")
			Expect(len(job.Spec.Template.Spec.Volumes)).To(Equal(1))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal(configMapVolumeName))
			Expect(job.Spec.Template.Spec.Volumes[0].ConfigMap).ToNot(BeNil())
			Expect(job.Spec.Template.Spec.Volumes[0].ConfigMap.Name).To(Equal("testMap"))
			Expect(len(job.Spec.Template.Spec.Containers[0].VolumeMounts)).To(Equal(1))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(configMapVolumeName))
		})

		It("should create a volume and mount for each volume that belongs to the VM", func() {
			vmSpec.Spec.Template.Spec.Volumes = []kubevirtv1.Volume{
				{
					Name: "volume1",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-1"},
					},
				},
				{
					Name: "volume2",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-2"},
					},
				},
			}
			job := MakeGuestConversionJobSpec(vmSpec, configMap, "")
			Expect(len(job.Spec.Template.Spec.Volumes)).To(Equal(3))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal("dv-1"))
			Expect(job.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("dv-1"))

			Expect(job.Spec.Template.Spec.Volumes[1].Name).To(Equal("dv-2"))
			Expect(job.Spec.Template.Spec.Volumes[1].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("dv-2"))

			Expect(job.Spec.Template.Spec.Volumes[2].Name).To(Equal(configMapVolumeName))
			Expect(job.Spec.Template.Spec.Volumes[2].ConfigMap).ToNot(BeNil())
			Expect(job.Spec.Template.Spec.Volumes[2].ConfigMap.Name).To(Equal("testMap"))

			Expect(len(job.Spec.Template.Spec.Containers[0].VolumeMounts)).To(Equal(3))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("dv-1"))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/mnt/disks/disk0"))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[1].Name).To(Equal("dv-2"))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[1].MountPath).To(Equal("/mnt/disks/disk1"))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name).To(Equal(configMapVolumeName))
		})
	})

	Describe("MakeLibvirtDomain", func() {
		var vmSpec *kubevirtv1.VirtualMachine
		var volumes []kubevirtv1.Volume

		BeforeEach(func() {
			volumes = []kubevirtv1.Volume{
				{
					Name: "volume1",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-1"},
					},
				},
				{
					Name: "volume2",
					VolumeSource: kubevirtv1.VolumeSource{
						DataVolume: &kubevirtv1.DataVolumeSource{Name: "dv-2"},
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
		})

		It("should create a DomainDisk for each volume on the VM", func() {
			domain := MakeLibvirtDomain(vmSpec)
			Expect(len(domain.Devices.Disks)).To(Equal(2))
			Expect(domain.Devices.Disks[0].Source.File.File).To(Equal("/mnt/disks/disk0/disk.img"))
			Expect(domain.Devices.Disks[0].Target.Dev).To(Equal("hda"))
			Expect(domain.Devices.Disks[1].Source.File.File).To(Equal("/mnt/disks/disk1/disk.img"))
			Expect(domain.Devices.Disks[1].Target.Dev).To(Equal("hdb"))
		})
	})
})
