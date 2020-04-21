package ovirtprovider

import (
	"errors"
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/configmaps"

	"github.com/kubevirt/vm-import-operator/pkg/secrets"

	"github.com/kubevirt/vm-import-operator/pkg/utils"

	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mappings"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtclient "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/client"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
	otemplates "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/templates"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	templates "github.com/kubevirt/vm-import-operator/pkg/templates"
	templatev1 "github.com/openshift/api/template/v1"
	tempclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ovirtLabel     = "oVirt"
	ovirtSecretKey = "ovirt"
	keyAccessKey   = "accessKeyId"
	keySecretKey   = "secretKey"
	diskNameFormat = "disk-%v"
)

var (
	labels = map[string]string{
		"origin": ovirtLabel,
	}
)

// SecretsManager defines operations on secrets
type SecretsManager interface {
	FindFor(types.NamespacedName) (*corev1.Secret, error)
	CreateFor(*corev1.Secret, types.NamespacedName) error
	DeleteFor(types.NamespacedName) error
}

// ConfigMapsManager defines operations on config maps
type ConfigMapsManager interface {
	FindFor(types.NamespacedName) (*corev1.ConfigMap, error)
	CreateFor(*corev1.ConfigMap, types.NamespacedName) error
	DeleteFor(types.NamespacedName) error
}

// OvirtProvider is Ovirt implementation of the Provider interface to support importing VM from ovirt
type OvirtProvider struct {
	ovirtSecretDataMap map[string]string
	ovirtClient        ovirtclient.OvirtClient
	validator          validation.VirtualMachineImportValidator
	vm                 *ovirtsdk.Vm
	vmiCrName          types.NamespacedName
	resourceMapping    *v2vv1alpha1.OvirtMappings
	templateFinder     *otemplates.TemplateFinder
	templateHandler    *templates.TemplateHandler
	secretsManager     SecretsManager
	configMapsManager  ConfigMapsManager
}

// NewOvirtProvider creates new OvirtProvider configured with dependencies
func NewOvirtProvider(vmiCrName types.NamespacedName, client client.Client, tempClient *tempclient.TemplateV1Client) OvirtProvider {
	validator := validators.NewValidatorWrapper(client)
	secretsManager := secrets.NewManager(client)
	configMapsManager := configmaps.NewManager(client)
	templateProvider := templates.NewTemplateProvider(tempClient)
	return OvirtProvider{
		vmiCrName:         vmiCrName,
		validator:         validation.NewVirtualMachineImportValidator(validator),
		templateFinder:    otemplates.NewTemplateFinder(templateProvider, templates.NewOSMapProvider(client)),
		templateHandler:   templates.NewTemplateHandler(templateProvider),
		secretsManager:    &secretsManager,
		configMapsManager: &configMapsManager,
	}
}

// GetVMStatus provides source VM status
func (o *OvirtProvider) GetVMStatus() (provider.VMStatus, error) {
	if status, ok := o.vm.Status(); ok {
		switch status {
		case ovirtsdk.VMSTATUS_DOWN:
			return provider.VMStatusDown, nil
		case ovirtsdk.VMSTATUS_UP:
			return provider.VMStatusUp, nil
		}
	}
	return "", fmt.Errorf("VM doesn't have a legal status. Allowed statuses: [%v, %v]", ovirtsdk.VMSTATUS_UP, ovirtsdk.VMSTATUS_DOWN)
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
func (o *OvirtProvider) Validate() ([]v2vv1alpha1.VirtualMachineImportCondition, error) {
	if o.vm == nil {
		return []v2vv1alpha1.VirtualMachineImportCondition{}, errors.New("VM has not been loaded")
	}
	return o.validator.Validate(o.vm, &o.vmiCrName, o.resourceMapping), nil
}

// StopVM stop the source VM on ovirt
func (o *OvirtProvider) StopVM() error {
	vmID, _ := o.vm.Id()
	status, _ := o.vm.Status()
	if status == ovirtsdk.VMSTATUS_DOWN {
		return nil
	}
	err := o.ovirtClient.StopVM(vmID)
	if err != nil {
		return err
	}
	return nil
}

// FindTemplate attempts to find best match for a template based on the source VM
func (o *OvirtProvider) FindTemplate() (*templatev1.Template, error) {
	return o.templateFinder.FindTemplate(o.vm)
}

// ProcessTemplate uses openshift api to process template
func (o *OvirtProvider) ProcessTemplate(template *templatev1.Template, vmName *string) (*kubevirtv1.VirtualMachine, error) {
	return o.templateHandler.ProcessTemplate(template, vmName)
}

// CreateMapper create the mapper for ovirt provider
func (o *OvirtProvider) CreateMapper() (provider.Mapper, error) {
	credentials, err := o.prepareDataVolumeCredentials()
	if err != nil {
		return nil, err
	}
	return mapper.NewOvirtMapper(o.vm, o.resourceMapping, credentials, o.vmiCrName.Namespace), nil
}

// UpdateVM updates VM specification with data volumes information
func (o *OvirtProvider) UpdateVM(vmspec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume) {
	// Volumes definition:
	volumes := make([]kubevirtv1.Volume, len(dvs))
	i := 0
	for _, dv := range dvs {
		volumes[i] = kubevirtv1.Volume{
			Name: fmt.Sprintf(diskNameFormat, i),
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
			Name: fmt.Sprintf(diskNameFormat, i),
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
	vmspec.Spec.Template.Spec.Domain.Devices.Disks = disks
	vmspec.Spec.Template.Spec.Volumes = volumes
}

// StartVM starts the source VM
func (o *OvirtProvider) StartVM() error {
	if id, ok := o.vm.Id(); ok {
		err := o.ovirtClient.StartVM(id)
		if err != nil {
			return err
		}
	}
	return nil
}

// CleanUp removes transient resources created for import
func (o *OvirtProvider) CleanUp() error {
	var errs []error
	err := o.secretsManager.DeleteFor(o.vmiCrName)
	if err != nil {
		errs = append(errs, err)
	}

	err = o.configMapsManager.DeleteFor(o.vmiCrName)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return foldErrors(errs, o.vmiCrName)
	}
	return nil
}

func (o *OvirtProvider) prepareDataVolumeCredentials() (mapper.DataVolumeCredentials, error) {
	keyAccess := o.ovirtSecretDataMap["username"]
	keySecret := o.ovirtSecretDataMap["password"]
	secret, err := o.ensureSecretIsPresent(keyAccess, keySecret)
	if err != nil {
		return mapper.DataVolumeCredentials{}, err
	}

	caCert := o.ovirtSecretDataMap["caCert"]
	configMap, err := o.ensureConfigMapIsPresent(caCert)
	if err != nil {
		return mapper.DataVolumeCredentials{}, err
	}

	return mapper.DataVolumeCredentials{
		URL:           o.ovirtSecretDataMap["apiUrl"],
		CACertificate: caCert,
		KeyAccess:     keyAccess,
		KeySecret:     keySecret,
		ConfigMapName: configMap.Name,
		SecretName:    secret.Name,
	}, nil
}

func (o *OvirtProvider) ensureSecretIsPresent(keyAccess string, keySecret string) (*corev1.Secret, error) {
	secret, err := o.secretsManager.FindFor(o.vmiCrName)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		secret, err = o.createSecret(keyAccess, keySecret)
		if err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func (o *OvirtProvider) createSecret(keyAccess string, keySecret string) (*corev1.Secret, error) {
	newSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.vmiCrName.Namespace,
		},
		Data: map[string][]byte{
			keyAccessKey: []byte(keyAccess),
			keySecretKey: []byte(keySecret),
		},
	}
	err := o.secretsManager.CreateFor(&newSecret, o.vmiCrName)
	if err != nil {
		return nil, err
	}
	return &newSecret, nil
}

func (o *OvirtProvider) ensureConfigMapIsPresent(caCert string) (*corev1.ConfigMap, error) {
	configMap, err := o.configMapsManager.FindFor(o.vmiCrName)
	if err != nil {
		return nil, err
	}
	if configMap == nil {
		configMap, err = o.createConfigMap(caCert)
		if err != nil {
			return nil, err
		}
	}
	return configMap, nil
}

func (o *OvirtProvider) createConfigMap(caCert string) (*corev1.ConfigMap, error) {
	newConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.vmiCrName.Namespace,
		},
		Data: map[string]string{
			"ca.pem": caCert,
		},
	}
	err := o.configMapsManager.CreateFor(&newConfigMap, o.vmiCrName)
	if err != nil {
		return nil, err
	}
	return &newConfigMap, nil
}

func getDiskAttachmentByID(id string, diskAttachments *ovirtsdk.DiskAttachmentSlice) *ovirtsdk.DiskAttachment {
	for _, diskAttachment := range diskAttachments.Slice() {
		if diskAttachment.MustId() == id {
			return diskAttachment
		}
	}
	return nil
}

func foldErrors(errs []error, vmiName types.NamespacedName) error {
	message := ""
	for _, e := range errs {
		message = utils.WithMessage(message, e.Error())
	}
	return fmt.Errorf("clean-up for %v failed: %s", utils.ToLoggableResourceName(vmiName.Name, &vmiName.Namespace), message)
}
