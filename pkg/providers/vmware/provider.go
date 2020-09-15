package vmware

import (
	libOS "os"
	"encoding/xml"
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/configmaps"
	"github.com/kubevirt/vm-import-operator/pkg/jobs"
	oapiv1 "github.com/openshift/api/template/v1"
	tempclient "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/client-go/api/v1"
	libvirtxml "libvirt.org/libvirt-go-xml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
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

var (
	virtV2vImage = libOS.Getenv("VIRTV2V_IMAGE")
	imagePullPolicy = corev1.PullPolicy(libOS.Getenv("IMAGE_PULL_POLICY"))
)

// VmwareProvider is VMware implementation of the Provider interface to support importing VMs from VMware
type VmwareProvider struct {
	dataVolumesManager    provider.DataVolumesManager
	factory               pclient.Factory
	instance              *v1beta1.VirtualMachineImport
	osFinder              *vos.VmwareOSFinder
	resourceMapping       *v1beta1.VmwareMappings
	secretsManager        provider.SecretsManager
	configMapsManager     provider.ConfigMapsManager
	jobsManager           provider.JobsManager
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
func NewVmwareProvider(vmiObjectMeta metav1.ObjectMeta, vmiTypeMeta metav1.TypeMeta, client client.Client, tempClient *tempclient.TemplateV1Client, factory pclient.Factory, ctrlConfig ctrlConfig.ControllerConfig) VmwareProvider {
	secretsManager := secrets.NewManager(client)
	configMapsManager := configmaps.NewManager(client)
	dataVolumesManager := datavolumes.NewManager(client)
	virtualMachineManager := virtualmachines.NewManager(client)
	jobsManager := jobs.NewManager(client)
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
		jobsManager:           &jobsManager,
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
	if _, ok := r.vmwareSecretDataMap["apiUrl"]; !ok {
		return fmt.Errorf("vmware secret must contain apiUrl attribute")
	}
	if len(r.vmwareSecretDataMap["apiUrl"]) == 0 {
		return fmt.Errorf("vmware secret apiUrl cannot be empty")
	}
	if _, ok := r.vmwareSecretDataMap["username"]; !ok {
		return fmt.Errorf("vmware secret must contain username attribute")
	}
	if len(r.vmwareSecretDataMap["username"]) == 0 {
		return fmt.Errorf("vmware secret username cannot be empty")
	}
	if _, ok := r.vmwareSecretDataMap["password"]; !ok {
		return fmt.Errorf("vmware secret must contain password attribute")
	}
	if len(r.vmwareSecretDataMap["password"]) == 0 {
		return fmt.Errorf("vmware secret password cannot be empty")
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
	utils.UpdateLabels(vm, labels)
	utils.UpdateAnnotations(vm, annotations)
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
	vm, err := r.getVmProperties()
	if err != nil {
		return "", err
	}
	return vm.Name, nil
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

// StopVM powers off the source VM.
func (r *VmwareProvider) StopVM(instance *v1beta1.VirtualMachineImport, client client.Client) error {
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
		err = utils.AddFinalizer(instance, utils.RestoreVMStateFinalizer, client)
		if err != nil {
			return err
		}
		return nil
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

	// only clean up the job on success,
	// since the job log is important for debugging
	if !failure {
		err = r.jobsManager.DeleteFor(vmiName)
		if err != nil {
			errs = append(errs, err)
		}
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
		return utils.FoldCleanUpErrors(errs, vmiName)
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
		conditions.NewCondition(v1beta1.MappingRulesVerified, string(v1beta1.MappingRulesVerificationCompleted), "All mapping rules checks passed", corev1.ConditionTrue),
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
	vmiName := r.getNamespacedName()
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
	vmiName := r.getNamespacedName()
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

func (r *VmwareProvider) NeedsGuestConversion() bool {
	return true
}

func (r *VmwareProvider) GetGuestConversionJob() (*batchv1.Job, error) {
	vmiName := r.getNamespacedName()
	job, err := r.jobsManager.FindFor(vmiName)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *VmwareProvider) LaunchGuestConversionJob(vmSpec *v1.VirtualMachine) (*batchv1.Job, error) {
	configMap, err := r.ensureConfigMapIsPresent(vmSpec)
	if err != nil {
		return nil, err
	}
	return r.ensureGuestConversionJobIsPresent(vmSpec, configMap)
}

func (r *VmwareProvider) ensureConfigMapIsPresent(vmSpec *v1.VirtualMachine) (*corev1.ConfigMap, error) {
	vmiName := r.getNamespacedName()
	configMap, err := r.configMapsManager.FindFor(vmiName)
	if err != nil {
		return nil, err
	}
	if configMap == nil {
		configMap, err = r.createConfigMap(vmSpec)
		if err != nil {
			return nil, err
		}
	}
	return configMap, nil
}

func (r *VmwareProvider) createConfigMap(vmSpec *v1.VirtualMachine) (*corev1.ConfigMap, error) {
	vmiName := r.getNamespacedName()
	domain := makeLibvirtDomain(vmSpec)
	domXML, err := xml.Marshal(domain)
	if err != nil {
		return nil, err
	}
	newConfigMap := &corev1.ConfigMap{
		BinaryData: map[string][]byte{
			"input.xml": domXML,
		},
	}
	newConfigMap.OwnerReferences = []metav1.OwnerReference{
		ownerreferences.NewVMImportOwnerReference(r.vmiTypeMeta, r.vmiObjectMeta),
	}
	err = r.configMapsManager.CreateFor(newConfigMap, vmiName)
	if err != nil {
		return nil, err
	}
	return newConfigMap, nil
}

func (r *VmwareProvider) ensureGuestConversionJobIsPresent(vmSpec *v1.VirtualMachine, libvirtConfigMap *corev1.ConfigMap) (*batchv1.Job, error) {
	vmiName := r.getNamespacedName()
	job, err := r.jobsManager.FindFor(vmiName)
	if err != nil {
		return nil, err
	}
	if job == nil {
		job, err = r.createGuestConversionJob(vmSpec, libvirtConfigMap)
		if err != nil {
			return nil, err
		}
	}
	return job, nil
}

func (r *VmwareProvider) createGuestConversionJob(vmSpec *v1.VirtualMachine, libvirtConfigMap *corev1.ConfigMap) (*batchv1.Job, error) {
	vmiName := r.getNamespacedName()
	job := makeGuestConversionJobSpec(vmSpec, libvirtConfigMap)
	job.OwnerReferences = []metav1.OwnerReference{
		ownerreferences.NewVMImportOwnerReference(r.vmiTypeMeta, r.vmiObjectMeta),
	}
	err := r.jobsManager.CreateFor(job, vmiName)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func makeGuestConversionJobSpec(vmSpec *v1.VirtualMachine, libvirtConfigMap *corev1.ConfigMap) *batchv1.Job {
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
						},
					},
					Volumes: volumes,
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
		Name: vmSpec.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: libvirtConfigMap.Name,
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      vmSpec.Name,
		MountPath: "/mnt/v2v",
	})
	return volumes, volumeMounts
}

// makeLibvirtDomain makes a minimal libvirt domain for a VM to be used by the guest conversion job
func makeLibvirtDomain(vmSpec *v1.VirtualMachine) *libvirtxml.Domain {
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

func (r *VmwareProvider) getNamespacedName() k8stypes.NamespacedName {
	return k8stypes.NamespacedName{
		Name:      r.vmiObjectMeta.Name,
		Namespace: r.vmiObjectMeta.Namespace,
	}
}