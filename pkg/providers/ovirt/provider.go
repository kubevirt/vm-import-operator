package ovirtprovider

import (
	"errors"
	"fmt"
	"strings"

	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"

	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"

	"github.com/kubevirt/vm-import-operator/pkg/ownerreferences"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/vm-import-operator/pkg/os"

	oos "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/os"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	"github.com/kubevirt/vm-import-operator/pkg/configmaps"
	"github.com/kubevirt/vm-import-operator/pkg/datavolumes"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mapper"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/mappings"
	otemplates "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/templates"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	"github.com/kubevirt/vm-import-operator/pkg/secrets"
	templates "github.com/kubevirt/vm-import-operator/pkg/templates"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	"github.com/kubevirt/vm-import-operator/pkg/virtualmachines"
	templatev1 "github.com/openshift/api/template/v1"
	tempclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
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

// OvirtProvider is Ovirt implementation of the Provider interface to support importing VM from ovirt
type OvirtProvider struct {
	ovirtSecretDataMap    map[string]string
	ovirtClient           pclient.VMClient
	validator             validation.VirtualMachineImportValidator
	vm                    *ovirtsdk.Vm
	vmiObjectMeta         metav1.ObjectMeta
	vmiTypeMeta           metav1.TypeMeta
	resourceMapping       *v2vv1.OvirtMappings
	osFinder              oos.OSFinder
	templateFinder        *otemplates.TemplateFinder
	templateHandler       *templates.TemplateHandler
	secretsManager        provider.SecretsManager
	configMapsManager     provider.ConfigMapsManager
	datavolumesManager    provider.DataVolumesManager
	virtualMachineManager provider.VirtualMachineManager
	factory               pclient.Factory
	instance              *v2vv1.VirtualMachineImport
}

// NewOvirtProvider creates new OvirtProvider configured with dependencies
func NewOvirtProvider(vmiObjectMeta metav1.ObjectMeta, vmiTypeMeta metav1.TypeMeta, client client.Client, tempClient *tempclient.TemplateV1Client, factory pclient.Factory, kvConfigProvider kvConfig.KubeVirtConfigProvider, ctrlConfig ctrlConfig.ControllerConfig) OvirtProvider {
	validator := validators.NewValidatorWrapper(client, kvConfigProvider)
	secretsManager := secrets.NewManager(client)
	configMapsManager := configmaps.NewManager(client)
	datavolumesManager := datavolumes.NewManager(client)
	virtualMachineManager := virtualmachines.NewManager(client)
	templateProvider := templates.NewTemplateProvider(tempClient)
	osFinder := oos.OVirtOSFinder{OsMapProvider: os.NewOSMapProvider(client, ctrlConfig.OsConfigMapName(), ctrlConfig.OsConfigMapNamespace())}
	return OvirtProvider{
		vmiObjectMeta:         vmiObjectMeta,
		vmiTypeMeta:           vmiTypeMeta,
		validator:             validation.NewVirtualMachineImportValidator(validator),
		osFinder:              &osFinder,
		templateFinder:        otemplates.NewTemplateFinder(templateProvider, &osFinder),
		templateHandler:       templates.NewTemplateHandler(templateProvider),
		secretsManager:        &secretsManager,
		configMapsManager:     &configMapsManager,
		datavolumesManager:    &datavolumesManager,
		virtualMachineManager: &virtualMachineManager,
		factory:               factory,
	}
}

// GetVMStatus provides source VM status
func (o *OvirtProvider) GetVMStatus() (provider.VMStatus, error) {
	vm, err := o.getVM()
	if err != nil {
		return "", err
	}
	if status, ok := vm.Status(); ok {
		switch status {
		case ovirtsdk.VMSTATUS_DOWN:
			return provider.VMStatusDown, nil
		case ovirtsdk.VMSTATUS_UP:
			return provider.VMStatusUp, nil
		}
	}
	return "", fmt.Errorf("VM doesn't have a legal status. Allowed statuses: [%v, %v]", ovirtsdk.VMSTATUS_UP, ovirtsdk.VMSTATUS_DOWN)
}

// Init ovirt provider using given secret
func (o *OvirtProvider) Init(secret *corev1.Secret, instance *v2vv1.VirtualMachineImport) error {
	o.ovirtSecretDataMap = make(map[string]string)
	err := yaml.Unmarshal(secret.Data[ovirtSecretKey], &o.ovirtSecretDataMap)
	if err != nil {
		return err
	}
	if _, ok := o.ovirtSecretDataMap["apiUrl"]; !ok {
		return fmt.Errorf("oVirt secret must contain apiUrl attribute")
	}
	if len(o.ovirtSecretDataMap["apiUrl"]) == 0 {
		return fmt.Errorf("oVirt secret apiUrl cannot be empty")
	}
	if _, ok := o.ovirtSecretDataMap["username"]; !ok {
		return fmt.Errorf("oVirt secret must contain username attribute")
	}
	if len(o.ovirtSecretDataMap["username"]) == 0 {
		return fmt.Errorf("oVirt secret username cannot be empty")
	}
	if _, ok := o.ovirtSecretDataMap["password"]; !ok {
		return fmt.Errorf("oVirt secret must contain password attribute")
	}
	if len(o.ovirtSecretDataMap["password"]) == 0 {
		return fmt.Errorf("oVirt secret password cannot be empty")
	}
	if _, ok := o.ovirtSecretDataMap["caCert"]; !ok {
		return fmt.Errorf("oVirt secret must contain caCert attribute")
	}
	if len(o.ovirtSecretDataMap["caCert"]) == 0 {
		return fmt.Errorf("oVirt secret caCert cannot be empty")
	}
	o.instance = instance
	return nil
}

// TestConnection tests the connection to ovirt provider
func (o *OvirtProvider) TestConnection() error {
	client, err := o.getClient()
	if err != nil {
		return err
	}
	err = client.TestConnection()
	if err != nil {
		return err
	}
	return nil
}

func (o *OvirtProvider) getClient() (pclient.VMClient, error) {
	if o.ovirtClient == nil {
		client, err := o.factory.NewOvirtClient(o.ovirtSecretDataMap)
		if err != nil {
			return nil, err
		}
		o.ovirtClient = client
	}
	return o.ovirtClient, nil
}

func (o *OvirtProvider) getVM() (*ovirtsdk.Vm, error) {
	if o.vm == nil {
		err := o.LoadVM(o.instance.Spec.Source)
		if err != nil {
			return nil, err
		}
	}
	return o.vm, nil
}

// GetVMName return oVirt virtual machine to be imported
func (o *OvirtProvider) GetVMName() (string, error) {
	vm, err := o.getVM()
	if err != nil {
		return "", err
	}
	vmName, _ := vm.Name()
	return vmName, nil
}

// Close the connection to ovirt provider
func (o *OvirtProvider) Close() {
	if o.ovirtClient != nil {
		o.ovirtClient.Close()
	}
}

// LoadVM fetch the source VM from ovirt and set it on the provider
func (o *OvirtProvider) LoadVM(sourceSpec v2vv1.VirtualMachineImportSourceSpec) error {
	ovirtSourceSpec := sourceSpec.Ovirt
	sourceVMID := ovirtSourceSpec.VM.ID
	sourceVMName := ovirtSourceSpec.VM.Name
	var sourceVMClusterName *string
	var sourceVMClusterID *string
	if ovirtSourceSpec.VM.Cluster != nil {
		sourceVMClusterName = ovirtSourceSpec.VM.Cluster.Name
		sourceVMClusterID = ovirtSourceSpec.VM.Cluster.ID
	}
	client, err := o.getClient()
	if err != nil {
		return err
	}
	vm, err := client.GetVM(sourceVMID, sourceVMName, sourceVMClusterName, sourceVMClusterID)
	if err != nil {
		return err
	}
	o.vm = vm.(*ovirtsdk.Vm)
	return nil
}

// PrepareResourceMapping merges external resource mapping and resource mapping provided in the virtual machine import spec
func (o *OvirtProvider) PrepareResourceMapping(externalResourceMapping *v2vv1.ResourceMappingSpec, vmiSpec v2vv1.VirtualMachineImportSourceSpec) {
	o.resourceMapping = mappings.MergeMappings(externalResourceMapping, vmiSpec.Ovirt.Mappings)
}

// ValidateDiskStatus validate current status of the disk in oVirt env:
func (o *OvirtProvider) ValidateDiskStatus(diskName string) (bool, error) {
	// Refresh cached VM data:
	err := o.LoadVM(o.instance.Spec.Source)
	if err != nil {
		return false, err
	}

	// Find the disk by ID and validate the status:
	if diskAttachments, ok := o.vm.DiskAttachments(); ok {
		for _, disk := range diskAttachments.Slice() {
			if diskID, ok := disk.Id(); ok {
				if strings.Contains(diskName, diskID) {
					return o.validator.Validator.ValidateDiskStatus(*disk), nil
				}
			}
		}
	}

	return false, nil
}

// Validate validates whether loaded previously VM and resource mapping is valid. The validation results are recorded in th VMI CR identified by vmiCrName and in case of a validation failure error is returned.
func (o *OvirtProvider) Validate() ([]v2vv1.VirtualMachineImportCondition, error) {
	vm, err := o.getVM()
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return []v2vv1.VirtualMachineImportCondition{}, errors.New("VM has not been loaded")
	}
	vmiName := o.GetVmiNamespacedName()
	return o.validator.Validate(vm, &vmiName, o.resourceMapping), nil
}

// StopVM stop the source VM on ovirt
func (o *OvirtProvider) StopVM() error {
	vm, err := o.getVM()
	if err != nil {
		return err
	}
	vmID, _ := vm.Id()
	status, _ := vm.Status()
	if status == ovirtsdk.VMSTATUS_DOWN {
		return nil
	}
	client, err := o.getClient()
	if err != nil {
		return err
	}
	err = client.StopVM(vmID)
	if err != nil {
		return err
	}
	return nil
}

// FindTemplate attempts to find best match for a template based on the source VM
func (o *OvirtProvider) FindTemplate() (*templatev1.Template, error) {
	vm, err := o.getVM()
	if err != nil {
		return nil, err
	}
	return o.templateFinder.FindTemplate(vm)
}

// ProcessTemplate uses openshift api to process template
func (o *OvirtProvider) ProcessTemplate(template *templatev1.Template, vmName *string, namespace string) (*kubevirtv1.VirtualMachine, error) {
	vm, err := o.templateHandler.ProcessTemplate(template, vmName, namespace)
	if err != nil {
		return nil, err
	}
	sourceVM, err := o.getVM()
	if err != nil {
		return nil, err
	}
	labels, annotations, err := o.templateFinder.GetMetadata(template, sourceVM)
	if err != nil {
		return nil, err
	}
	utils.UpdateLabels(vm, labels)
	utils.UpdateAnnotations(vm, annotations)
	return vm, nil
}

// GetVmiNamespacedName return the namespaced name of the VM import object
func (o *OvirtProvider) GetVmiNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: o.vmiObjectMeta.Name, Namespace: o.vmiObjectMeta.Namespace}
}

// CreateMapper create the mapper for ovirt provider
func (o *OvirtProvider) CreateMapper() (provider.Mapper, error) {
	credentials, err := o.prepareDataVolumeCredentials()
	if err != nil {
		return nil, err
	}
	vm, err := o.getVM()
	if err != nil {
		return nil, err
	}
	return mapper.NewOvirtMapper(vm, o.resourceMapping, credentials, o.vmiObjectMeta.Namespace, o.osFinder), nil
}

// StartVM starts the source VM
func (o *OvirtProvider) StartVM() error {
	vm, err := o.getVM()
	if err != nil {
		return err
	}
	if id, ok := vm.Id(); ok {
		client, err := o.getClient()
		if err != nil {
			return err
		}
		err = client.StartVM(id)
		if err != nil {
			return err
		}
	}
	return nil
}

// CleanUp removes transient resources created for import
func (o *OvirtProvider) CleanUp(failure bool, cr *v2vv1.VirtualMachineImport, client rclient.Client) error {
	var errs []error

	err := utils.RemoveFinalizer(cr, utils.RestoreVMStateFinalizer, client)
	if err != nil {
		errs = append(errs, err)
	}

	vmiName := o.GetVmiNamespacedName()
	err = o.secretsManager.DeleteFor(vmiName)
	if err != nil {
		errs = append(errs, err)
	}

	err = o.configMapsManager.DeleteFor(vmiName)
	if err != nil {
		errs = append(errs, err)
	}

	if failure {
		err = o.datavolumesManager.DeleteFor(vmiName)
		if err != nil {
			errs = append(errs, err)
		}

		err = o.virtualMachineManager.DeleteFor(vmiName)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return utils.FoldCleanUpErrors(errs, vmiName)
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
	secret, err := o.secretsManager.FindFor(o.GetVmiNamespacedName())
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
		Data: map[string][]byte{
			keyAccessKey: []byte(keyAccess),
			keySecretKey: []byte(keySecret),
		},
	}
	newSecret.OwnerReferences = []metav1.OwnerReference{
		ownerreferences.NewVMImportOwnerReference(o.vmiTypeMeta, o.vmiObjectMeta),
	}
	err := o.secretsManager.CreateFor(&newSecret, o.GetVmiNamespacedName())
	if err != nil {
		return nil, err
	}
	return &newSecret, nil
}

func (o *OvirtProvider) ensureConfigMapIsPresent(caCert string) (*corev1.ConfigMap, error) {
	configMap, err := o.configMapsManager.FindFor(o.GetVmiNamespacedName())
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
		Data: map[string]string{
			"ca.pem": caCert,
		},
	}
	newConfigMap.OwnerReferences = []metav1.OwnerReference{
		ownerreferences.NewVMImportOwnerReference(o.vmiTypeMeta, o.vmiObjectMeta),
	}

	err := o.configMapsManager.CreateFor(&newConfigMap, o.GetVmiNamespacedName())
	if err != nil {
		return nil, err
	}
	return &newConfigMap, nil
}