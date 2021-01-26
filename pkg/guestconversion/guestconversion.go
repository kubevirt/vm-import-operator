package guestconversion

import (
	"fmt"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"os"

	"kubevirt.io/containerized-data-importer/pkg/common"

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

// MakeGuestConversionPodSpec creates a pod spec for a virt-v2v pod,
// containing a volume and a mount for each volume on the VM, as well
// as a volume and mount for the config map containing the libvirt domain XML.
func MakeGuestConversionPodSpec(vmSpec *v1.VirtualMachine, dataVolumes map[string]cdiv1.DataVolume, libvirtConfigMap *corev1.ConfigMap) *corev1.Pod {
	// this is the fsGroup that the CDI importer pod uses
	fsGroup := common.QemuSubGid

	volumes, volumeMounts, volumeDevices := makePodVolumeMounts(vmSpec, dataVolumes, libvirtConfigMap)

	return &corev1.Pod{
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: &fsGroup,
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "virt-v2v",
					Image:           virtV2vImage,
					VolumeMounts:    volumeMounts,
					VolumeDevices:   volumeDevices,
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
	}
}

func makePodVolumeMounts(vmSpec *v1.VirtualMachine, dataVolumes map[string]cdiv1.DataVolume, libvirtConfigMap *corev1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount, []corev1.VolumeDevice) {
	volumes := make([]corev1.Volume, 0)
	volumeMounts := make([]corev1.VolumeMount, 0)
	volumeDevices := make([]corev1.VolumeDevice, 0)

	// add volumes and mounts for each of the VM's disks.
	// the virt-v2v pod expects to see the disks mounted at /mnt/disks/diskX
	for i, v := range vmSpec.Spec.Template.Spec.Volumes {
		var volumeMode corev1.PersistentVolumeMode
		dv, ok := dataVolumes[v.DataVolume.Name]
		if ok && dv.Spec.PVC != nil && dv.Spec.PVC.VolumeMode != nil {
			volumeMode = *dv.Spec.PVC.VolumeMode
		} else {
			// default to Filesystem if a volume mode is not specified
			volumeMode = corev1.PersistentVolumeFilesystem
		}

		vol := corev1.Volume{
			Name: dv.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: dv.Name,
					ReadOnly:  false,
				},
			},
		}
		volumes = append(volumes, vol)

		if volumeMode == corev1.PersistentVolumeBlock {
			volDevice := corev1.VolumeDevice{
				Name:       v.DataVolume.Name,
				DevicePath: fmt.Sprintf("/dev/block%v", i),
			}
			volumeDevices = append(volumeDevices, volDevice)
		} else {
			volMount := corev1.VolumeMount{
				Name:      v.DataVolume.Name,
				MountPath: fmt.Sprintf("/mnt/disks/disk%v", i),
			}
			volumeMounts = append(volumeMounts, volMount)
		}
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
	return volumes, volumeMounts, volumeDevices
}

// MakeLibvirtDomain makes a minimal libvirt domain for a VM to be used by the guest conversion pod
func MakeLibvirtDomain(vmSpec *v1.VirtualMachine, dataVolumes map[string]cdiv1.DataVolume) *libvirtxml.Domain {
	// virt-v2v needs a very minimal libvirt domain XML file to be provided
	// with the locations of each of the disks on the VM that is to be converted.
	libvirtDisks := make([]libvirtxml.DomainDisk, 0)
	for i, vol := range vmSpec.Spec.Template.Spec.Volumes {
		diskSource := libvirtxml.DomainDiskSource{}

		dv := dataVolumes[vol.DataVolume.Name]
		if *dv.Spec.PVC.VolumeMode == corev1.PersistentVolumeBlock {
			diskSource.Block = &libvirtxml.DomainDiskSourceBlock{
				Dev: fmt.Sprintf("/dev/block%v", i),
			}
		} else {
			diskSource.File = &libvirtxml.DomainDiskSourceFile{
				// the location where the disk images will be found on
				// the virt-v2v pod. See also makePodVolumeMounts.
				File: fmt.Sprintf("/mnt/disks/disk%v/disk.img", i),
			}
		}

		libvirtDisk := libvirtxml.DomainDisk{
			Device: "disk",
			Driver: &libvirtxml.DomainDiskDriver{
				Name: "qemu",
				Type: "raw",
			},
			Source: &diskSource,
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
