package vmware

import (
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/configmaps"
	oapiv1 "github.com/openshift/api/template/v1"
	tempclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"
	"github.com/kubevirt/vm-import-operator/pkg/datavolumes"
	"github.com/kubevirt/vm-import-operator/pkg/os"
	"github.com/kubevirt/vm-import-operator/pkg/ownerreferences"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	vclient "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/client"
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/mapper"
	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware/mappings"
	vos "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/os"
	vtemplates "github.com/kubevirt/vm-import-operator/pkg/providers/vmware/templates"
	"github.com/kubevirt/vm-import-operator/pkg/secrets"
	"github.com/kubevirt/vm-import-operator/pkg/templates"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	"github.com/kubevirt/vm-import-operator/pkg/virtualmachines"
)

const (
	apiUrlKey       = "apiUrl"
	usernameKey     = "username"
	passwordKey     = "password"
	keyAccessKey    = "accessKeyId"
	keySecretKey    = "secretKey"
	thumbprintKey   = "thumbprint"
	vmwareSecretKey = "vmware"
)

type VmwareProvider struct {
	dataVolumesManager    provider.DataVolumesManager
	factory               pclient.Factory
	instance              *v1beta1.VirtualMachineImport
	osFinder              *vos.VmwareOSFinder
	resourceMapping       *v1beta1.VmwareMappings
	secretsManager        provider.SecretsManager
	configMapsManager     provider.ConfigMapsManager
	templateFinder        *vtemplates.TemplateFinder
	templateHandler       *templates.TemplateHandler
	virtualMachineManager provider.VirtualMachineManager
	vm                    *object.VirtualMachine
	vmProperties          *mo.VirtualMachine
	vmiObjectMeta         metav1.ObjectMeta
	vmiTypeMeta           metav1.TypeMeta
	vmwareClient          *vclient.RichVmwareClient
	vmwareSecretDataMap   map[string]string
}

// NewVmwareProvider creates a new VmwareProvider
func NewVmwareProvider(vmiObjectMeta metav1.ObjectMeta, vmiTypeMeta metav1.TypeMeta, client client.Client, tempClient *tempclient.TemplateV1Client, factory pclient.Factory,
	_ kvConfig.KubeVirtConfigProvider, ctrlConfig ctrlConfig.ControllerConfig) VmwareProvider {
	secretsManager := secrets.NewManager(client)
	configMapsManager := configmaps.NewManager(client)
	dataVolumesManager := datavolumes.NewManager(client)
	virtualMachineManager := virtualmachines.NewManager(client)
	templateProvider := templates.NewTemplateProvider(tempClient)
	osFinder := vos.VmwareOSFinder{OsMapProvider: os.NewOSMapProvider(client, ctrlConfig.OsConfigMapName(), ctrlConfig.OsConfigMapNamespace())}
	return VmwareProvider{
		vmiObjectMeta:         vmiObjectMeta,
		vmiTypeMeta:           vmiTypeMeta,
		factory:               factory,
		secretsManager:        &secretsManager,
		configMapsManager:     &configMapsManager,
		dataVolumesManager:    &dataVolumesManager,
		virtualMachineManager: &virtualMachineManager,
		osFinder:              &osFinder,
		templateHandler:       templates.NewTemplateHandler(templateProvider),
		templateFinder:        vtemplates.NewTemplateFinder(templateProvider, osFinder),
	}
}

// Init initializes the VmwareProvider with a given credential secret and VirtualMachineImport
func (r *VmwareProvider) Init(secret *corev1.Secret, instance *v1beta1.VirtualMachineImport) error {
	r.vmwareSecretDataMap = make(map[string]string)
	err := yaml.Unmarshal(secret.Data[vmwareSecretKey], &r.vmwareSecretDataMap)
	if err != nil {
		return err
	}
	r.instance = instance
	return nil
}

// CreateMapper creates a VM mapper for this provider.
func (r *VmwareProvider) CreateMapper() (provider.Mapper, error) {
	credentials, err := r.prepareDataVolumeCredentials()
	if err != nil {
		return nil, err
	}
	vmwareClient, err := r.getClient()
	if err != nil {
		return nil, err
	}
	vm, err := r.getVM()
	if err != nil {
		return nil, err
	}
	vmProperties, err := r.getVmProperties()
	if err != nil {
		return nil, err
	}
	hostProperties, err := vmwareClient.GetVMHostProperties(vm)
	if err != nil {
		return nil, err
	}
	return mapper.NewVmwareMapper(vm, vmProperties, hostProperties, credentials, r.resourceMapping, r.vmiObjectMeta.Namespace, r.osFinder), nil
}

// FindTemplate attempts to find best match for a template based on the source VM
func (r *VmwareProvider) FindTemplate() (*oapiv1.Template, error) {
	vm, err := r.getVmProperties()
	if err != nil {
		return nil, err
	}
	return r.templateFinder.FindTemplate(vm)
}

// ProcessTemplate uses the Openshift API to process a template
func (r *VmwareProvider) ProcessTemplate(template *oapiv1.Template, vmName *string, namespace string) (*v1.VirtualMachine, error) {
	vm, err := r.templateHandler.ProcessTemplate(template, vmName, namespace)
	if err != nil {
		return nil, err
	}
	vmProperties, err := r.getVmProperties()
	if err != nil {
		return nil, err
	}
	labels, annotations, err := r.templateFinder.GetMetadata(template, vmProperties)
	if err != nil {
		return nil, err
	}
	updateLabels(vm, labels)
	updateAnnotations(vm, annotations)
	return vm, nil
}

// PrepareResourceMapping merges the external resource mapping with the mapping provided in the VirtualMachineImport spec
func (r *VmwareProvider) PrepareResourceMapping(externalResourceMapping *v1beta1.ResourceMappingSpec, vmiSpec v1beta1.VirtualMachineImportSourceSpec) {
	r.resourceMapping = mappings.MergeMappings(externalResourceMapping, vmiSpec.Vmware.Mappings)
}

// LoadVM fetches the source VM.
func (r *VmwareProvider) LoadVM(sourceSpec v1beta1.VirtualMachineImportSourceSpec) error {
	vmwareClient, err := r.getClient()
	if err != nil {
		return err
	}
	vm, err := vmwareClient.GetVM(sourceSpec.Vmware.VM.ID, sourceSpec.Vmware.VM.Name, nil, nil)
	if err != nil {
		return err
	}
	r.vm = vm.(*object.VirtualMachine)
	return nil
}

// GetVMName gets the name of the source VM
func (r *VmwareProvider) GetVMName() (string, error) {
	vm, err := r.getVM()
	if err != nil {
		return "", err
	}
	return vm.Name(), nil
}

// GetVMStatus gets the power status of the source VM
func (r *VmwareProvider) GetVMStatus() (provider.VMStatus, error) {
	vmProperties, err := r.getVmProperties()
	if err != nil {
		return "", err
	}

	poweredOn := types.VirtualMachinePowerStatePoweredOn
	poweredOff := types.VirtualMachinePowerStatePoweredOff

	switch vmProperties.Runtime.PowerState {
	case poweredOn:
		return provider.VMStatusUp, nil
	case poweredOff:
		return provider.VMStatusDown, nil
	}

	return "", fmt.Errorf("VM doesn't have a legal status. Allowed statuses: [%v, %v]", poweredOn, poweredOff)
}

// StartVM powers on the source VM.
func (r *VmwareProvider) StartVM() error {
	vmwareClient, err := r.getClient()
	if err != nil {
		return err
	}
	vm, err := r.getVM()
	if err != nil {
		return err
	}
	return vmwareClient.StartVM(vm.Reference().Value)
}

// StartVM powers off the source VM.
func (r *VmwareProvider) StopVM(cr *v1beta1.VirtualMachineImport, client client.Client) error {
	vmwareClient, err := r.getClient()
	if err != nil {
		return err
	}
	vm, err := r.getVM()
	if err != nil {
		return err
	}
	vmProperties, err := r.getVmProperties()
	if err != nil {
		return err
	}

	if vmProperties.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOff {
		err = vmwareClient.StopVM(vm.Reference().Value)
		if err != nil {
			return err
		}
	}

	err = utils.AddFinalizer(cr, utils.RestoreVMStateFinalizer, client)
	if err != nil {
		return err
	}
	return nil
}

// CleanUp removes transient resources created for import
func (r *VmwareProvider) CleanUp(failure bool, cr *v1beta1.VirtualMachineImport, client client.Client) error {
	var errs []error

	err := utils.RemoveFinalizer(cr, utils.RestoreVMStateFinalizer, client)
	if err != nil {
		errs = append(errs, err)
	}

	vmiName := k8stypes.NamespacedName{
		Name:      r.vmiObjectMeta.Name,
		Namespace: r.vmiObjectMeta.Namespace,
	}

	err = r.secretsManager.DeleteFor(vmiName)
	if err != nil {
		errs = append(errs, err)
	}

	err = r.configMapsManager.DeleteFor(vmiName)
	if err != nil {
		errs = append(errs, err)
	}

	if failure {
		err = r.dataVolumesManager.DeleteFor(vmiName)
		if err != nil {
			errs = append(errs, err)
		}

		err = r.virtualMachineManager.DeleteFor(vmiName)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return foldErrors(errs, vmiName)
	}
	return nil
}

// TestConnection tests the connection to the vCenter or ESXi host.
func (r *VmwareProvider) TestConnection() error {
	vmwareClient, err := r.getClient()
	if err != nil {
		return err
	}
	return vmwareClient.TestConnection()
}

// Validate checks whether the source VM and resource mapping is valid.
func (r *VmwareProvider) Validate() ([]v1beta1.VirtualMachineImportCondition, error) {
	// TODO: implement vmware rule validation
	return []v1beta1.VirtualMachineImportCondition{
		conditions.NewCondition(v1beta1.Valid, string(v1beta1.ValidationCompleted), "Validation completed successfully", corev1.ConditionTrue),
	}, nil
}

// Close is a no-op which is present in order to satisfy the Provider interface.
func (r *VmwareProvider) Close() {
	// nothing to do
}

// ValidateDiskStatus is a no-op which is present in order to satisfy the Provider interface.
func (r *VmwareProvider) ValidateDiskStatus(_ string) (bool, error) {
	return true, nil
}

func (r *VmwareProvider) getClient() (*vclient.RichVmwareClient, error) {
	if r.vmwareClient == nil {
		c, err := r.factory.NewVmwareClient(r.vmwareSecretDataMap)
		if err != nil {
			return nil, err
		}
		r.vmwareClient = c.(*vclient.RichVmwareClient)
	}
	return r.vmwareClient, nil
}

func (r *VmwareProvider) getVM() (*object.VirtualMachine, error) {
	if r.vm == nil {
		err := r.LoadVM(r.instance.Spec.Source)
		if err != nil {
			return nil, err
		}
	}
	return r.vm, nil
}

func (r *VmwareProvider) getVmProperties() (*mo.VirtualMachine, error) {
	vmwareClient, err := r.getClient()
	if err != nil {
		return nil, err
	}
	vm, err := r.getVM()
	if err != nil {
		return nil, err
	}
	if r.vmProperties == nil {
		properties, err := vmwareClient.GetVMProperties(vm)
		if err != nil {
			return nil, err
		}
		r.vmProperties = properties
	}
	return r.vmProperties, nil
}

func (r *VmwareProvider) prepareDataVolumeCredentials() (*mapper.DataVolumeCredentials, error) {
	username := r.vmwareSecretDataMap[usernameKey]
	password := r.vmwareSecretDataMap[passwordKey]

	secret, err := r.ensureSecretIsPresent(username, password)
	if err != nil {
		return &mapper.DataVolumeCredentials{}, err
	}

	return &mapper.DataVolumeCredentials{
		URL:        r.vmwareSecretDataMap[apiUrlKey],
		Thumbprint: r.vmwareSecretDataMap[thumbprintKey],
		Username:   username,
		Password:   password,
		SecretName: secret.Name,
	}, nil
}

func (r *VmwareProvider) ensureSecretIsPresent(keyAccess, keySecret string) (*corev1.Secret, error) {
	vmiName := k8stypes.NamespacedName{
		Name:      r.vmiObjectMeta.Name,
		Namespace: r.vmiObjectMeta.Namespace,
	}
	secret, err := r.secretsManager.FindFor(vmiName)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		secret, err = r.createSecret(keyAccess, keySecret)
		if err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func (r *VmwareProvider) createSecret(username, password string) (*corev1.Secret, error) {
	vmiName := k8stypes.NamespacedName{
		Name:      r.vmiObjectMeta.Name,
		Namespace: r.vmiObjectMeta.Namespace,
	}
	newSecret := corev1.Secret{
		Data: map[string][]byte{
			keyAccessKey: []byte(username),
			keySecretKey: []byte(password),
		},
	}
	newSecret.OwnerReferences = []metav1.OwnerReference{
		ownerreferences.NewVMImportOwnerReference(r.vmiTypeMeta, r.vmiObjectMeta),
	}
	err := r.secretsManager.CreateFor(&newSecret, vmiName)
	if err != nil {
		return nil, err
	}
	return &newSecret, nil
}

func foldErrors(errs []error, vmiName k8stypes.NamespacedName) error {
	message := ""
	for _, e := range errs {
		message = utils.WithMessage(message, e.Error())
	}
	return fmt.Errorf("clean-up for %v failed: %s", utils.ToLoggableResourceName(vmiName.Name, &vmiName.Namespace), message)
}

func updateLabels(vm *v1.VirtualMachine, labels map[string]string) {
	utils.AppendMap(vm.ObjectMeta.GetLabels(), labels)
	utils.AppendMap(vm.Spec.Template.ObjectMeta.GetLabels(), labels)
}

func updateAnnotations(vm *v1.VirtualMachine, annotationMap map[string]string) {
	annotations := vm.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		vm.ObjectMeta.SetAnnotations(annotations)
	}
	utils.AppendMap(annotations, annotationMap)
}
