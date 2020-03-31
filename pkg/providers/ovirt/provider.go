package ovirtprovider

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mappings"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtclient "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/client"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	ovirtsdk "github.com/ovirt/go-ovirt"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ovirtSecret    = "ovirt-key"
	ovirtConfigmap = "ovirt-ca"
	ovirtLabel     = "oVirt"
	ovirtSecretKey = "ovirt"
)

var (
	labels = map[string]string{
		"origin": ovirtLabel,
	}
)

// OvirtProvider is Ovirt implementation of the Provider interface to support importing VM from ovirt
type OvirtProvider struct {
	ovirtSecretDataMap map[string]string
	ovirtClient        ovirtclient.OvirtClient
	validator          validation.VirtualMachineImportValidator
	vm                 *ovirtsdk.Vm
	vmiCrName          types.NamespacedName
	resourceMapping    *v2vv1alpha1.OvirtMappings
}

// NewOvirtProvider creates new OvirtProvider configured with dependencies
func NewOvirtProvider(vmiCrName types.NamespacedName, client client.Client, kubevirtClient kubecli.KubevirtClient) OvirtProvider {
	validator := validators.NewValidatorWrapper(kubevirtClient)
	provider := OvirtProvider{
		vmiCrName: vmiCrName,
		validator: validation.NewVirtualMachineImportValidator(client, validator),
	}
	return provider
}

// GetDataVolumeCredentials returns the data volume credentials based on ovirt secret
func (o *OvirtProvider) GetDataVolumeCredentials() provider.DataVolumeCredentials {
	suffix := createSuffix(o.ovirtSecretDataMap["apiUrl"])
	return provider.DataVolumeCredentials{
		URL:           o.ovirtSecretDataMap["apiUrl"],
		CACertificate: o.ovirtSecretDataMap["caCert"],
		KeyAccess:     o.ovirtSecretDataMap["username"],
		KeySecret:     o.ovirtSecretDataMap["password"],

		// TODO: name of the two attributes should be unique per vmimport cr
		// assuming we wish to GC all of the resources related to this cr
		ConfigMapName: ovirtSecret + suffix,
		SecretName:    ovirtConfigmap + suffix,
	}
}

func createSuffix(s string) string {
	md5HashInBytes := md5.Sum([]byte(s))
	md5HashInString := hex.EncodeToString(md5HashInBytes[:])
	return "-" + md5HashInString[0:8]
}

// Connect to ovirt provider using given secret
func (o *OvirtProvider) Connect(secret *corev1.Secret) error {
	o.ovirtSecretDataMap = make(map[string]string)
	err := yaml.Unmarshal(secret.Data[ovirtSecretKey], &o.ovirtSecretDataMap)
	if err != nil {
		return err
	}
	if _, ok := o.ovirtSecretDataMap["caCert"]; !ok {
		return fmt.Errorf("oVirt secret must contain caCert attribute")
	}
	if len(o.ovirtSecretDataMap["caCert"]) == 0 {
		return fmt.Errorf("oVirt secret caCert cannot be empty")
	}
	o.ovirtClient, err = ovirtclient.NewRichOvirtClient(
		&ovirtclient.ConnectionSettings{
			URL:      o.ovirtSecretDataMap["apiUrl"],
			Username: o.ovirtSecretDataMap["username"],
			Password: o.ovirtSecretDataMap["password"],
			CACert:   []byte(o.ovirtSecretDataMap["caCert"]),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

// Close the connection to ovirt provider
func (o *OvirtProvider) Close() {
	if o.ovirtClient != nil {
		o.ovirtClient.Close()
	}
}

// LoadVM fetch the source VM from ovirt and set it on the provider
func (o *OvirtProvider) LoadVM(sourceSpec v2vv1alpha1.VirtualMachineImportSourceSpec) error {
	ovirtSourceSpec := sourceSpec.Ovirt
	sourceVMID := ovirtSourceSpec.VM.ID
	sourceVMName := ovirtSourceSpec.VM.Name
	var sourceVMClusterName *string
	var sourceVMClusterID *string
	if ovirtSourceSpec.VM.Cluster != nil {
		sourceVMClusterName = ovirtSourceSpec.VM.Cluster.Name
		sourceVMClusterID = ovirtSourceSpec.VM.Cluster.ID
	}
	vm, err := o.ovirtClient.GetVM(sourceVMID, sourceVMName, sourceVMClusterName, sourceVMClusterID)
	if err != nil {
		return err
	}
	o.vm = vm
	return nil
}

// PrepareResourceMapping merges external resource mapping and resource mapping provided in the virtual machine import spec
func (o *OvirtProvider) PrepareResourceMapping(externalResourceMapping *v2vv1alpha1.ResourceMappingSpec, vmiSpec v2vv1alpha1.VirtualMachineImportSourceSpec) {
	o.resourceMapping = mappings.MergeMappings(externalResourceMapping, vmiSpec.Ovirt.Mappings)
}

// Validate validates whether loaded previously VM and resource mapping is valid. The validation results are recorded in th VMI CR identified by vmiCrName and in case of a validation failure error is returned.
func (o *OvirtProvider) Validate() error {
	if o.vm == nil {
		return errors.New("VM has not been loaded")
	}
	return o.validator.Validate(o.vm, &o.vmiCrName, o.resourceMapping)
}

// StopVM stop the source VM on ovirt
func (o *OvirtProvider) StopVM() error {
	vmID, _ := o.vm.Id()
	err := o.ovirtClient.StopVM(vmID)
	if err != nil {
		return err
	}
	return nil
}

// CreateMapper create the mapper for ovirt provider
func (o *OvirtProvider) CreateMapper() provider.Mapper {
	return mapper.NewOvirtMapper(o.vm, o.resourceMapping, o.GetDataVolumeCredentials(), o.vmiCrName.Namespace)
}

// UpdateVM updates VM specification with data volumes information
func (o *OvirtProvider) UpdateVM(vmspec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume) {
	// Volumes definition:
	volumes := make([]kubevirtv1.Volume, len(dvs))
	i := 0
	for _, dv := range dvs {
		volumes[i] = kubevirtv1.Volume{
			Name: fmt.Sprintf("dv-%v", i),
			VolumeSource: kubevirtv1.VolumeSource{
				DataVolume: &kubevirtv1.DataVolumeSource{
					Name: dv.Name,
				},
			},
		}
		i++
	}
	// Disks definition:
	i = 0
	disks := make([]kubevirtv1.Disk, len(dvs))
	for id := range dvs {
		diskAttachment := getDiskAttachmentByID(id, o.vm.MustDiskAttachments())
		disks[i] = kubevirtv1.Disk{
			Name: fmt.Sprintf("dv-%v", i),
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: string(diskAttachment.MustInterface()),
				},
			},
		}
		if diskAttachment.MustBootable() {
			bootOrder := uint(1)
			disks[i].BootOrder = &bootOrder
		}

		i++
	}
	running := false
	vmspec.Spec = kubevirtv1.VirtualMachineSpec{
		Running: &running,
		Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: kubevirtv1.VirtualMachineInstanceSpec{
				Domain: kubevirtv1.DomainSpec{
					CPU: &kubevirtv1.CPU{
						Cores: uint32(o.vm.MustCpu().MustTopology().MustCores()),
					},
					Devices: kubevirtv1.Devices{
						Disks: disks,
						// Memory:  &kubevirtv1.Memory{},
						// Machine:   kubevirtv1.Machine{},
						// Firmware:  &kubevirtv1.Firmware{},
						// Clock:     &kubevirtv1.Clock{},
						// Features:  &kubevirtv1.Features{},
						// Chassis:   &kubevirtv1.Chassis{},
						// IOThreadsPolicy: &kubevirtv1.IOThreadsPolicy{},
					},
				},
				Volumes: volumes,
			},
		},
	}
}

func getDiskAttachmentByID(id string, diskAttachments *ovirtsdk.DiskAttachmentSlice) *ovirtsdk.DiskAttachment {
	for _, diskAttachment := range diskAttachments.Slice() {
		if diskAttachment.MustId() == id {
			return diskAttachment
		}
	}
	return nil
}
