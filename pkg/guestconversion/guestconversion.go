package guestconversion

import (
	"fmt"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "kubevirt.io/client-go/api/v1"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

const configMapVolumeName = "libvirt-domain-xml"

var (
	virtV2vImage    = os.Getenv("VIRTV2V_IMAGE")
	imagePullPolicy = corev1.PullPolicy(os.Getenv("IMAGE_PULL_POLICY"))
)

// MakeGuestConversionJobSpec creates a job spec for a virt-v2v job,
// containing a volume and a mount for each volume on the VM, as well
// as a volume and mount for the config map containing the libvirt domain XML.
func MakeGuestConversionJobSpec(vmSpec *v1.VirtualMachine, libvirtConfigMap *corev1.ConfigMap) *batchv1.Job {
	// Only ever run the guest conversion job once per VM
	completions := int32(1)
	parallelism := int32(1)
	backoffLimit := int32(0)

	volumes, volumeMounts := makeJobVolumeMounts(vmSpec, libvirtConfigMap)

	return &batchv1.Job{
		Spec: batchv1.JobSpec{
			Completions:  &completions,
			Parallelism:  &parallelism,
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "virt-v2v",
							Image:           virtV2vImage,
							VolumeMounts:    volumeMounts,
							ImagePullPolicy: imagePullPolicy,
							// Request access to /dev/kvm via Kubevirt's Device Manager
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"devices.kubevirt.io/kvm": resource.MustParse("1"),
								},
							},
						},
					},
					Volumes: volumes,
					// Ensure that the pod is deployed on a node where /dev/kvm is present.
					NodeSelector: map[string]string{
						"kubevirt.io/schedulable": "true",
					},
				},
			},
		},
		Status: batchv1.JobStatus{},
	}
}

func makeJobVolumeMounts(vmSpec *v1.VirtualMachine, libvirtConfigMap *corev1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := make([]corev1.Volume, 0)
	volumeMounts := make([]corev1.VolumeMount, 0)
	// add volumes and mounts for each of the VM's disks.
	// the virt-v2v pod expects to see the disks mounted at /mnt/disks/diskX
	for i, dataVolume := range vmSpec.Spec.Template.Spec.Volumes {
		vol := corev1.Volume{
			Name: dataVolume.DataVolume.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: dataVolume.DataVolume.Name,
					ReadOnly:  false,
				},
			},
		}
		volumes = append(volumes, vol)

		volMount := corev1.VolumeMount{
			Name:      dataVolume.DataVolume.Name,
			MountPath: fmt.Sprintf("/mnt/disks/disk%v", i),
		}
		volumeMounts = append(volumeMounts, volMount)
	}

	// add volume and mount for the libvirt domain xml config map.
	// the virt-v2v pod expects to see the libvirt xml at /mnt/v2v/input.xml
	volumes = append(volumes, corev1.Volume{
		Name: configMapVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: libvirtConfigMap.Name,
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      configMapVolumeName,
		MountPath: "/mnt/v2v",
	})
	return volumes, volumeMounts
}

// MakeLibvirtDomain makes a minimal libvirt domain for a VM to be used by the guest conversion job
func MakeLibvirtDomain(vmSpec *v1.VirtualMachine) *libvirtxml.Domain {
	// virt-v2v needs a very minimal libvirt domain XML file to be provided
	// with the locations of each of the disks on the VM that is to be converted.
	libvirtDisks := make([]libvirtxml.DomainDisk, 0)
	for i := range vmSpec.Spec.Template.Spec.Volumes {
		libvirtDisk := libvirtxml.DomainDisk{
			Device: "disk",
			Driver: &libvirtxml.DomainDiskDriver{
				Name: "qemu",
				Type: "raw",
			},
			Source: &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					// the location where the disk images will be found on
					// the virt-v2v pod. See also makeJobVolumeMounts.
					File: fmt.Sprintf("/mnt/disks/disk%v/disk.img", i),
				},
			},
			Target: &libvirtxml.DomainDiskTarget{
				Dev: "hd" + string(rune('a'+i)),
				Bus: "virtio",
			},
		}
		libvirtDisks = append(libvirtDisks, libvirtDisk)
	}

	// generate libvirt domain xml
	domain := vmSpec.Spec.Template.Spec.Domain
	return &libvirtxml.Domain{
		Type: "kvm",
		Name: vmSpec.Name,
		Memory: &libvirtxml.DomainMemory{
			Value: uint(domain.Resources.Requests.Memory().Value()),
		},
		CPU: &libvirtxml.DomainCPU{
			Topology: &libvirtxml.DomainCPUTopology{
				Sockets: int(domain.CPU.Sockets),
				Cores:   int(domain.CPU.Cores),
			},
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type: "hvm",
			},
			BootDevices: []libvirtxml.DomainBootDevice{
				{
					Dev: "hd",
				},
			},
		},
		Devices: &libvirtxml.DomainDeviceList{
			Disks: libvirtDisks,
		},
	}
}
