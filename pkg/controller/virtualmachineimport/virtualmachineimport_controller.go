package virtualmachineimport

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware"

	"github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/resources"

	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"

	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"

	"github.com/kubevirt/vm-import-operator/pkg/metrics"
	libvirtxml "libvirt.org/libvirt-go-xml"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
	"github.com/kubevirt/vm-import-operator/pkg/ownerreferences"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtprovider "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	templatev1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	AnnAPIGroup          = "vmimport.v2v.kubevirt.io"
	sourceVMInitialState = AnnAPIGroup + "/source-vm-initial-state"
	// AnnCurrentProgress is annotations storing current progress of the vm import
	AnnCurrentProgress = AnnAPIGroup + "/progress"
	// AnnPropagate is annotation defining which values to propagate
	AnnPropagate = AnnAPIGroup + "/propagate-annotations"
	// TrackingLabel is a label used to track related entities.
	TrackingLabel = AnnAPIGroup + "/tracker"
	// VMLabel is a label used to track resources created for a VM
	VMLabel = AnnAPIGroup + "/vm"
	// constants
	progressStart           = "0"
	progressCreatingVM      = "5"
	progressCopyingDisks    = "10"
	progressConvertingGuest = "70"
	progressStartVM         = "90"
	progressDone            = "100"
	progressForCopyDisk     = 65
	progressCopyDiskRange   = float64(progressForCopyDisk / 100.0)

	requeueAfterValidationFailureTime = 5 * time.Second
	podCrashLoopBackOff               = "CrashLoopBackOff"
	importPodName                     = "importer"
	virtV2vJobName                    = "virt-v2v"

	// EventImportScheduled is emitted when import scheduled
	EventImportScheduled = "ImportScheduled"
	// EventImportInProgress is emitted when import is in progress
	EventImportInProgress = "ImportInProgress"
	// EventImportSucceeded is emitted when import succeed
	EventImportSucceeded = "ImportSucceeded"
	// EventImportBlocked is emitted when import is blocked
	EventImportBlocked = "ImportBlocked"
	// EventVMStartFailed is emitted when vm failed to start
	EventVMStartFailed = "VMStartFailed"
	// EventVMCreationFailed is emitted when creation of vm fails
	EventVMCreationFailed = "VMCreationFailed"
	// EventDVCreationFailed is emitted when creation of datavolume fails
	EventDVCreationFailed = "DVCreationFailed"
	// EventPVCImportFailed is emitted when import of pvc fails
	EventPVCImportFailed = "EventPVCImportFailed"
	// EventGuestConversionFailed is emitted when the virt-v2v conversion job fails.
	EventGuestConversionFailed = "GuestConversionFailed"
)

var (
	log = logf.Log.WithName("controller_virtualmachineimport")
	// importPodRestartTolerance define how many restart of the import pod are tolerated before
	// we end the import as failed, by default it's 3.
	importPodRestartTolerance, _ = strconv.Atoi(os.Getenv("IMPORT_POD_RESTART_TOLERANCE"))
	virtV2vImage                 = os.Getenv("VIRTV2V_IMAGE")
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new VirtualMachineImport Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, kvConfigProvider kvConfig.KubeVirtConfigProvider, ctrlConfigProvider ctrlConfig.ControllerConfigProvider) error {
	return add(mgr, newReconciler(mgr, kvConfigProvider, ctrlConfigProvider))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kvConfigProvider kvConfig.KubeVirtConfigProvider, ctrlConfigProvider ctrlConfig.ControllerConfigProvider) *ReconcileVirtualMachineImport {
	tempClient, err := templatev1.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "Unable to get OC client")
		panic("Controller cannot operate without OC client")
	}
	reader := mgr.GetAPIReader()
	client := mgr.GetClient()
	finder := mappings.NewResourceMappingsFinder(client)
	ownerreferencesmgr := ownerreferences.NewOwnerReferenceManager(client)
	factory := pclient.NewSourceClientFactory()
	return &ReconcileVirtualMachineImport{client: client,
		apiReader:              reader,
		scheme:                 mgr.GetScheme(),
		resourceMappingsFinder: finder,
		ocClient:               tempClient,
		ownerreferencesmgr:     ownerreferencesmgr,
		factory:                factory,
		kvConfigProvider:       kvConfigProvider,
		ctrlConfigProvider:     ctrlConfigProvider,
		recorder:               mgr.GetEventRecorderFor("virtualmachineimport-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVirtualMachineImport) error {
	// Create a new controller
	c, err := controller.New("virtualmachineimport-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	r.controller = c

	// Watch for changes to primary resource VirtualMachineImport
	err = c.Watch(
		&source.Kind{Type: &v2vv1.VirtualMachineImport{}},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	// Watch for VM events:
	err = c.Watch(
		&source.Kind{Type: &kubevirtv1.VirtualMachine{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v2vv1.VirtualMachineImport{},
		},
	)
	if err != nil {
		return err
	}

	// Watch for DV events:
	err = c.Watch(
		&source.Kind{Type: &cdiv1.DataVolume{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v2vv1.VirtualMachineImport{},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &batchv1.Job{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v2vv1.VirtualMachineImport{},
		})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileVirtualMachineImport implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileVirtualMachineImport{}

// ReconcileVirtualMachineImport reconciles a VirtualMachineImport object
type ReconcileVirtualMachineImport struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client                 client.Client
	scheme                 *runtime.Scheme
	resourceMappingsFinder mappings.ResourceFinder
	ocClient               *templatev1.TemplateV1Client
	ownerreferencesmgr     ownerreferences.OwnerReferenceManager
	factory                pclient.Factory
	kvConfigProvider       kvConfig.KubeVirtConfigProvider
	ctrlConfigProvider     ctrlConfig.ControllerConfigProvider
	recorder               record.EventRecorder
	controller             controller.Controller
	apiReader              client.Reader
}

// Reconcile reads that state of the cluster for a VirtualMachineImport object and makes changes based on the state read
// and what is in the VirtualMachineImport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVirtualMachineImport) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling VirtualMachineImport")

	// Fetch the VirtualMachineImport instance
	instance := &v2vv1.VirtualMachineImport{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Init provider:
	provider, err := r.createProvider(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	message, err := r.initProvider(instance, provider)
	if err != nil {
		if r.vmImportInProgress(instance) {
			return reconcile.Result{}, err
		}
		// fail new request and don't requeue it if the provider couldn't be initialized properly
		err = r.failNewImportProcess(instance, message, err)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	defer provider.Close()

	if instance.DeletionTimestamp != nil && utils.HasFinalizer(instance, utils.RestoreVMStateFinalizer) {
		err := r.finalize(instance, provider)
		if err != nil {
			// requeue if locked
			return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return reconcile.Result{}, nil
	}

	// Exit if we should not run reconcile:
	if !shouldReconcile(instance) {
		reqLogger.Info("Not running reconcile")
		return reconcile.Result{}, nil
	}

	// fetch source vm
	err = r.fetchVM(instance, provider)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Validate if it's needed at this stage of processing
	valid, err := r.validate(instance, provider)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !valid {
		return reconcile.Result{RequeueAfter: requeueAfterValidationFailureTime}, nil
	}

	// Stop the VM
	if err = provider.StopVM(instance, r.client); err != nil {
		return reconcile.Result{}, err
	}

	// Create mapper:
	mapper, err := provider.CreateMapper()
	if err != nil {
		return reconcile.Result{}, err
	}

	vmName := types.NamespacedName{Name: instance.Status.TargetVMName, Namespace: request.Namespace}
	if instance.Status.TargetVMName == "" {
		newName, err := r.createVM(provider, instance, mapper)
		if err != nil {
			return reconcile.Result{}, err
		}
		vmName.Name = newName
		// Emit event we are starting the import process:
		r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportScheduled, "Import of Virtual Machine %s/%s started", vmName.Namespace, vmName.Name)
	}

	if shouldImportDisks(instance) {
		done, err := r.importDisks(provider, instance, mapper, vmName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !done {
			return reconcile.Result{}, nil
		}
	}

	if shouldConvertGuest(instance) {
		done, err := r.convertGuest(provider, instance, vmName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !done {
			return reconcile.Result{}, nil
		}
	}

	if !conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady, v2vv1.VirtualMachineRunning) {
		if err := r.updateConditionsAfterSuccess(instance, "Virtual machine disks import done", v2vv1.VirtualMachineReady); err != nil {
			return reconcile.Result{}, err
		}
	}

	if shouldStartVM(instance) {
		if err = r.startVM(provider, instance, vmName); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		// Update progress if all disks import done:
		if err := r.updateProgress(instance, progressDone); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.afterSuccess(vmName, provider, instance); err != nil {
			return reconcile.Result{}, err
		}

		// Emit event vm is successfully imported
		r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportSucceeded, "Virtual Machine %s/%s import successful", vmName.Namespace, vmName.Name)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVirtualMachineImport) finalize(instance *v2vv1.VirtualMachineImport, provider provider.Provider) error {
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	reqLogger.Info("Finalizing - restoring source vm")
	vmiName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}

	err := r.restoreInitialVMState(vmiName, provider)
	if err != nil {
		reqLogger.Error(err, "Finalizing - restoring failed")
		// disk locked is expected and we need to requeue
		if strings.Contains(err.Error(), "locked") {
			return err
		}
	}

	err = utils.RemoveFinalizer(instance, utils.RestoreVMStateFinalizer, r.client)
	if err != nil {
		reqLogger.Error(err, "Finalizing - failed to remove finalizer")
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) addWatchForImportPod(instance *v2vv1.VirtualMachineImport, dvID string) error {
	return r.controller.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
				if a.Meta.GetName() == importerPodNameFromDv(dvID) {
					return []reconcile.Request{
						{NamespacedName: types.NamespacedName{
							Name:      instance.Name,
							Namespace: instance.Namespace,
						}},
					}
				}
				return nil
			}),
		},
	)
}

// convertGuest starts a Job to run virt-v2v on the target VM
func (r *ReconcileVirtualMachineImport) convertGuest(provider provider.Provider, instance *v2vv1.VirtualMachineImport, vmName types.NamespacedName) (bool, error) {
	// find the vmspec
	vmSpec := &kubevirtv1.VirtualMachine{}
	err := r.client.Get(context.TODO(), vmName, vmSpec)
	if err != nil {
		return false, err
	}

	configMap, err := r.findLibvirtDomainConfigMap(vmName)
	if err != nil {
		return false, err
	}
	if configMap == nil {
		configMap, err = r.makeLibvirtDomainConfigMap(instance, vmSpec)
		if err != nil {
			return false, err
		}
	}

	job, err := r.findGuestConversionJob(vmName)
	if err != nil {
		return false, err
	}
	// the job doesn't exist, so create it
	if job == nil {
		job = r.makeGuestConversionJobSpec(vmSpec)
		// Set VirtualMachineImport instance as the owner and controller so it'll be cleaned up
		// when the instance is removed.
		if err := controllerutil.SetControllerReference(instance, job, r.scheme); err != nil {
			return false, err
		}
		err := r.client.Create(context.TODO(), job)
		if err != nil {
			return false, err
		}

		processingCond := conditions.NewProcessingCondition(string(v2vv1.ConvertingGuest), "Running virt-v2v", corev1.ConditionTrue)
		err = r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, processingCond)
		if err != nil {
			return false, err
		}

		// Update progress to converting guest:
		if err = r.updateProgress(instance, progressConvertingGuest); err != nil {
			return false, err
		}
	}

	if job.Status.Active > 0 {
		return false, nil
	}

	if job.Status.Failed > 0 {
		err := r.endGuestConversionAsFailed(provider, instance, "virt-v2v job failed")
		if err != nil {
			return false, err
		}
		return false, nil
	}

	return job.Status.Succeeded > 0, nil
}

// findGuestConversionJob finds the guest conversion job for a given VM, returning nil if it can't be found.
func (r *ReconcileVirtualMachineImport) findGuestConversionJob(vmName types.NamespacedName) (*batchv1.Job, error) {
	jobList := &batchv1.JobList{}
	matchingLabels := client.MatchingLabels(map[string]string{VMLabel: vmName.Name})
	err := r.client.List(context.TODO(), jobList, matchingLabels)
	if err != nil {
		return nil, err
	}
	if len(jobList.Items) > 0 {
		return &jobList.Items[0], nil
	}
	return nil, nil
}

func (r *ReconcileVirtualMachineImport) makeGuestConversionJobSpec(vmSpec *kubevirtv1.VirtualMachine) *batchv1.Job {
	// Only ever run the guest conversion job once per VM
	completions := int32(1)
	parallelism := int32(1)
	backoffLimit := int32(0)

	volumes, volumeMounts := makeJobVolumeMounts(vmSpec)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: virtV2vJobName + "-",
			Namespace:    vmSpec.Namespace,
			Labels: map[string]string{
				VMLabel: vmSpec.Name,
			},
		},
		Spec: batchv1.JobSpec{
			Completions:  &completions,
			Parallelism:  &parallelism,
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: virtV2vJobName + "-",
					Namespace:    vmSpec.Namespace,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Name:            virtV2vJobName,
							Image:           virtV2vImage,
							ImagePullPolicy: v1.PullIfNotPresent,
							VolumeMounts:    volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
		Status: batchv1.JobStatus{},
	}
}

func makeJobVolumeMounts(vmSpec *kubevirtv1.VirtualMachine) ([]v1.Volume, []v1.VolumeMount) {
	volumes := make([]v1.Volume, 0)
	volumeMounts := make([]v1.VolumeMount, 0)
	// add volumes and mounts for each of the VM's disks.
	// the virt-v2v pod expects to see the disks mounted at /mnt/disks/diskX
	for i, dataVolume := range vmSpec.Spec.Template.Spec.Volumes {
		vol := v1.Volume{
			Name: dataVolume.DataVolume.Name,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: dataVolume.DataVolume.Name,
					ReadOnly:  false,
				},
			},
		}
		volumes = append(volumes, vol)

		volMount := v1.VolumeMount{
			Name:      dataVolume.DataVolume.Name,
			MountPath: fmt.Sprintf("/mnt/disks/disk%v", i),
		}
		volumeMounts = append(volumeMounts, volMount)
	}

	// add volume and mount for the libvirt domain xml config map.
	// the virt-v2v pod expects to see the libvirt xml at /mnt/v2v/input.xml
	volumes = append(volumes, v1.Volume{
		Name: vmSpec.Name,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: vmSpec.Name,
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, v1.VolumeMount{
		Name:      vmSpec.Name,
		MountPath: "/mnt/v2v",
	})
	return volumes, volumeMounts
}

// findLibvirtDomainConfigMap finds the libvirt domain xml configmap for a given VM, returning nil if it can't be found.
func (r *ReconcileVirtualMachineImport) findLibvirtDomainConfigMap(vmName types.NamespacedName) (*v1.ConfigMap, error) {
	configMapList := &v1.ConfigMapList{}
	matchingLabels := client.MatchingLabels(map[string]string{VMLabel: vmName.Name})
	err := r.client.List(context.TODO(), configMapList, matchingLabels)
	if err != nil {
		return nil, err
	}
	if len(configMapList.Items) > 0 {
		return &configMapList.Items[0], nil
	}
	return nil, nil
}

// makeLibvirtDomainConfigMap creates a libvirt domain xml configmap for a VM to be used during guest conversion
func (r *ReconcileVirtualMachineImport) makeLibvirtDomainConfigMap(instance *v2vv1.VirtualMachineImport, vmSpec *kubevirtv1.VirtualMachine) (*v1.ConfigMap, error) {
	domain := r.makeLibvirtDomain(vmSpec)
	domxml, err := xml.Marshal(domain)
	if err != nil {
		return nil, err
	}
	domainXMLConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-libvirt-domain-", vmSpec.Name),
			Namespace:    vmSpec.Namespace,
			Labels: map[string]string{
				VMLabel: vmSpec.Name,
			},
		},
		BinaryData: map[string][]byte{
			"input.xml": domxml,
		},
	}
	err = controllerutil.SetOwnerReference(instance, domainXMLConfigMap, r.scheme)
	if err != nil {
		return nil, err
	}
	err = r.client.Create(context.TODO(), domainXMLConfigMap)
	if err != nil {
		return nil, err
	}
	return domainXMLConfigMap, nil
}

// makeLibvirtDomain makes a minimal libvirt domain for a VM to be used by the guest conversion job
func (r *ReconcileVirtualMachineImport) makeLibvirtDomain(vmSpec *kubevirtv1.VirtualMachine) *libvirtxml.Domain {
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

func (r *ReconcileVirtualMachineImport) importDisks(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName) (bool, error) {
	dvs, err := mapper.MapDataVolumes(&vmName.Name)
	if err != nil {
		return false, err
	}

	dvsDone := make(map[string]bool)
	dvsImportProgress := make(map[string]float64)
	for dvID, dv := range dvs {
		if err = r.addWatchForImportPod(instance, dvID); err != nil {
			return false, err
		}

		foundDv := &cdiv1.DataVolume{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: dvID}, foundDv)
		if err != nil && k8serrors.IsNotFound(err) {
			// We have to validate the disk status, so we are sure, the disk wasn't manipulated,
			// before we execute the import:
			valid, err := provider.ValidateDiskStatus(dv.Name)
			if err != nil {
				return false, err
			}
			if valid {
				if err = r.createDataVolume(provider, mapper, instance, dv, vmName); err != nil {
					if err = r.endImportAsFailed(provider, instance, foundDv, err.Error()); err != nil {
						return false, err
					}
					return false, err
				}
			} else {
				// If disk status is wrong, end the import as failed:
				if err = r.endImportAsFailed(provider, instance, foundDv, "disk is in illegal status"); err != nil {
					return false, err
				}
			}
		} else if err == nil {
			instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
			// Set dataVolume as done, if it's in Succeeded state:
			if foundDv.Status.Phase == cdiv1.Succeeded {
				dvsDone[dvID] = true
			} else if foundDv.Status.Phase == cdiv1.Failed {
				if err = r.endImportAsFailed(provider, instance, foundDv, "dv is in Failed Phase"); err != nil {
					return false, err
				}
			} else if foundDv.Status.Phase == cdiv1.Pending {
				// Update condition to pending the PVC bound:
				message := fmt.Sprintf("DataVolume %s is pending to bound", foundDv.Name)
				processingCond := conditions.NewProcessingCondition(string(v2vv1.Pending), message, corev1.ConditionTrue)
				if err := r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
					return false, err
				}
			} else if foundDv.Status.Phase == cdiv1.ImportInProgress {
				// Update condition to create copying disks:
				processingCond := conditions.NewProcessingCondition(string(v2vv1.CopyingDisks), "Copying virtual machine disks", corev1.ConditionTrue)
				err := r.upsertStatusConditions(instanceNamespacedName, processingCond)
				if err != nil {
					return false, err
				}

				// During ImportInProgress phase importer pod can be in crashloppbackoff, so we need
				// to check the state of the pod and fail the import:
				foundPod := &corev1.Pod{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: importerPodNameFromDv(dvID)}, foundPod)
				if err == nil {
					// Emit an event about why pod failed:
					if foundPod.Status.ContainerStatuses != nil &&
						foundPod.Status.ContainerStatuses[0].LastTerminationState.Terminated != nil &&
						foundPod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode > 0 {
						r.recorder.Eventf(instance, corev1.EventTypeWarning, EventPVCImportFailed, foundPod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Message)
					}
					// End the import in case the pod keeps crashing:
					for _, cs := range foundPod.Status.ContainerStatuses {
						if cs.State.Waiting != nil && cs.State.Waiting.Reason == podCrashLoopBackOff && cs.RestartCount > int32(importPodRestartTolerance) {
							if err = r.endImportAsFailed(provider, instance, foundDv, "pod CrashLoopBackoff restart exceeded"); err != nil {
								return false, err
							}
						}
					}
				}
			}
			// Get current progress of the import:
			progress := string(foundDv.Status.Progress)
			progressFloat, err := strconv.ParseFloat(strings.TrimRight(progress, "%"), 64)
			if err != nil {
				dvsImportProgress[dvID] = 0.0
			} else {
				dvsImportProgress[dvID] = progressFloat
			}
		} else {
			return false, err
		}
	}

	// Update progress:
	currentProgress := disksImportProgress(dvsImportProgress, float64(len(dvs)))
	if err := r.updateProgress(instance, currentProgress); err != nil {
		return false, err
	}

	done := r.isDoneImport(dvsDone, len(dvs))
	return done, nil
}

func (r *ReconcileVirtualMachineImport) endGuestConversionAsFailed(provider provider.Provider, instance *v2vv1.VirtualMachineImport, message string) error {
	errorMessage := fmt.Sprintf("Error converting guests: %s", message)
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}

	// Update processing condition to failed:
	processingCond := conditions.NewProcessingCondition(string(v2vv1.ProcessingFailed), errorMessage, corev1.ConditionFalse)
	if err := r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
		return err
	}

	// Update succeed condition to failed:
	succeededCond := conditions.NewSucceededCondition(string(v2vv1.GuestConversionFailed), errorMessage, corev1.ConditionFalse)
	if err := r.upsertStatusConditions(instanceNamespacedName, succeededCond); err != nil {
		return err
	}

	// Update progress to done.
	if err := r.updateProgress(instance, progressDone); err != nil {
		return err
	}

	// Update event:
	r.recorder.Event(instance, corev1.EventTypeWarning, EventGuestConversionFailed, message)

	// Cleanup
	if err := r.afterFailure(provider, instance); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) endImportAsFailed(provider provider.Provider, instance *v2vv1.VirtualMachineImport, dv *cdiv1.DataVolume, message string) error {
	errorMessage := fmt.Sprintf("Error while importing disk image: %s. %s", dv.Name, message)
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}

	// Update processing condition to failed:
	processingCond := conditions.NewProcessingCondition(string(v2vv1.ProcessingFailed), errorMessage, corev1.ConditionFalse)
	if err := r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
		return err
	}

	// Update succeed condition to failed:
	succeededCond := conditions.NewSucceededCondition(string(v2vv1.DataVolumeCreationFailed), errorMessage, corev1.ConditionFalse)
	if err := r.upsertStatusConditions(instanceNamespacedName, succeededCond); err != nil {
		return err
	}

	// Update progress to done.
	if err := r.updateProgress(instance, progressDone); err != nil {
		return err
	}

	// Update event:
	r.recorder.Event(instance, corev1.EventTypeWarning, EventDVCreationFailed, message)

	// Cleanup
	if err := r.afterFailure(provider, instance); err != nil {
		return err
	}

	return nil
}

func importerPodNameFromDv(dvID string) string {
	return fmt.Sprintf("%s-%s", importPodName, dvID)
}

func (r *ReconcileVirtualMachineImport) createVM(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper) (string, error) {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)

	// Resolve VM Name
	targetVMName := mapper.ResolveVMName(instance.Spec.TargetVMName)

	// Define VM spec
	// Update condition to VM template matching:
	processingCond := conditions.NewProcessingCondition(string(v2vv1.VMTemplateMatching), "Matching virtual machine template", corev1.ConditionTrue)
	if err := r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
		return "", err
	}
	template, err := provider.FindTemplate()
	var spec *kubevirtv1.VirtualMachine
	config, cfgErr := r.kvConfigProvider.GetConfig()
	if cfgErr != nil {
		log.Error(cfgErr, "Cannot get KubeVirt cluster config.")
	}
	if err != nil {
		reqLogger.Info("No matching template was found for the virtual machine.")
		if !config.ImportWithoutTemplateEnabled() {
			if err := r.templateMatchingFailed(err.Error(), &processingCond, provider, instance); err != nil {
				return "", err
			}
			return "", err
		}
		reqLogger.Info("Using empty VM definition.")
		spec = mapper.CreateEmptyVM(targetVMName)
	} else {
		reqLogger.Info("A template was found for creating the virtual machine", "Template.Name", template.ObjectMeta.Name)
		spec, err = provider.ProcessTemplate(template, targetVMName, instance.Namespace)
		if err != nil {
			reqLogger.Info("Failed to process the template. Error: " + err.Error())
			if !config.ImportWithoutTemplateEnabled() {
				return "", err
			}
			reqLogger.Info("Using empty VM definition.")
			spec = mapper.CreateEmptyVM(targetVMName)
		} else {
			if len(spec.ObjectMeta.Name) > 0 {
				targetVMName = &spec.ObjectMeta.Name
			}
		}
	}
	vmSpec, err := mapper.MapVM(targetVMName, spec)
	if err != nil {
		return "", err
	}

	// propagate annotations
	setAnnotations(instance, vmSpec)

	// propagate tracking label
	setTrackerLabel(vmSpec.ObjectMeta, instance)

	// Update progress:
	if err = r.updateProgress(instance, progressStart); err != nil {
		return "", err
	}

	// Set VirtualMachineImport instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, vmSpec, r.scheme); err != nil {
		return "", err
	}

	// Update condition to creating VM:
	processingCond = conditions.NewProcessingCondition(string(v2vv1.CreatingTargetVM), "Creating virtual machine", corev1.ConditionTrue)
	if err = r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
		return "", err
	}

	// Create kubevirt VM from source VM:
	reqLogger.Info("Creating a new VM", "VM.Namespace", vmSpec.Namespace, "VM.Name", vmSpec.Name)
	if err = r.client.Create(context.TODO(), vmSpec); err != nil && !k8serrors.IsAlreadyExists(err) {
		vmJSON, _ := json.Marshal(vmSpec)
		reqLogger.Info("VM struct", "VM spec", string(vmJSON))
		message := fmt.Sprintf("Error while creating virtual machine %s/%s: %s", vmSpec.Namespace, vmSpec.Name, err)
		// Update event:
		r.recorder.Event(instance, corev1.EventTypeWarning, EventVMCreationFailed, message)

		// Update condition to failed state:
		succeededCond := conditions.NewSucceededCondition(string(v2vv1.VMCreationFailed), message, corev1.ConditionFalse)
		processingCond.Status = corev1.ConditionFalse
		processingFailedReason := string(v2vv1.ProcessingFailed)
		processingCond.Reason = &processingFailedReason
		if err = r.upsertStatusConditions(instanceNamespacedName, succeededCond, processingCond); err != nil {
			return "", err
		}

		// Cleanup after failure
		if err = r.afterFailure(provider, instance); err != nil {
			return "", err
		}

		// Reconcile
		return "", err
	}

	// Get created VM Name
	found := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: vmSpec.Name, Namespace: vmSpec.Namespace}, found)
	if err != nil {
		return "", err
	}

	// Set target name:
	if err = r.updateTargetVMName(instanceNamespacedName, found.Name); err != nil {
		return "", err
	}

	// Update progress to creating vm
	if err = r.updateProgress(instance, progressCreatingVM); err != nil {
		return "", err
	}

	return found.Name, nil
}

func setAnnotations(instance *v2vv1.VirtualMachineImport, vmSpec *kubevirtv1.VirtualMachine) {
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	annotations := instance.GetAnnotations()
	propagate := annotations[AnnPropagate]
	if propagate != "" {
		annotations := vmSpec.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
			vmSpec.SetAnnotations(annotations)
		}
		prop := map[string]string{}
		err := json.Unmarshal([]byte(propagate), &prop)
		if err != nil {
			reqLogger.Info("Failed while parsing annotations to propagate to virtual machine")
			return
		}
		resources.WithLabels(annotations, prop)
	}
}

func setTrackerLabel(meta metav1.ObjectMeta, instance *v2vv1.VirtualMachineImport) {
	iLabels := instance.GetLabels()
	tracker := iLabels[TrackingLabel]
	if tracker != "" {
		labels := meta.GetLabels()
		if labels == nil {
			labels = map[string]string{}
			meta.SetLabels(labels)
		}
		labels[TrackingLabel] = tracker
	}
}

// startVM start the VM if was requested to be started and VM disks are imported and ready:
func (r *ReconcileVirtualMachineImport) startVM(provider provider.Provider, instance *v2vv1.VirtualMachineImport, vmName types.NamespacedName) error {
	vmi := &kubevirtv1.VirtualMachineInstance{}
	err := r.client.Get(context.TODO(), vmName, vmi)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if err = r.updateProgress(instance, progressStartVM); err != nil {
				return err
			}
			if err = r.updateToRunning(vmName); err != nil {
				// Emit event vm failed to start:
				r.recorder.Eventf(instance, corev1.EventTypeWarning, EventVMStartFailed, "Virtual Machine %s/%s failed to start: %s", vmName.Namespace, vmName.Name, err)
				return err
			}
		}
		return err
	}

	if vmi.Status.Phase == kubevirtv1.Running || vmi.Status.Phase == kubevirtv1.Scheduled {
		// Emit event vm is successfully imported and started:
		r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportSucceeded, "Virtual Machine %s/%s imported and started", vmName.Namespace, vmName.Name)

		if err = r.updateConditionsAfterSuccess(instance, "Virtual machine running", v2vv1.VirtualMachineRunning); err != nil {
			return err
		}
		if err = r.updateProgress(instance, progressDone); err != nil {
			return err
		}
		if err = r.afterSuccess(vmName, provider, instance); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) updateConditionsAfterSuccess(instance *v2vv1.VirtualMachineImport, message string, reason v2vv1.SucceededConditionReason) error {
	succeededCond := conditions.NewSucceededCondition(string(reason), message, corev1.ConditionTrue)
	conds := []v2vv1.VirtualMachineImportCondition{succeededCond}

	processingCond := conditions.FindConditionOfType(instance.Status.Conditions, v2vv1.Processing)
	if processingCond != nil {
		processingCond.Status = corev1.ConditionTrue
		processingCompletedReason := string(v2vv1.ProcessingCompleted)
		processingCond.Reason = &processingCompletedReason
		conds = append(conds, *processingCond)
	}

	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	return r.upsertStatusConditions(instanceNamespacedName, conds...)
}

func shouldReconcile(instance *v2vv1.VirtualMachineImport) bool {
	cond := conditions.FindConditionOfType(instance.Status.Conditions, v2vv1.Succeeded)

	// If VM is ready, but not yet started run reconcile
	if cond != nil {
		if (cond.Reason != nil && *cond.Reason == string(v2vv1.VirtualMachineReady)) && (instance.Spec.StartVM != nil && *instance.Spec.StartVM) {
			return true
		}
	}
	return cond == nil
}

func shouldStartVM(instance *v2vv1.VirtualMachineImport) bool {
	return instance.Spec.StartVM != nil && *instance.Spec.StartVM && conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady)
}

func shouldConvertGuest(instance *v2vv1.VirtualMachineImport) bool {
	return instance.Spec.Source.Vmware != nil && !conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady, v2vv1.VirtualMachineRunning)
}

func shouldImportDisks(instance *v2vv1.VirtualMachineImport) bool {
	return !conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady, v2vv1.VirtualMachineRunning)
}

// isDoneImport returns whether all disk imports are complete
func (r *ReconcileVirtualMachineImport) isDoneImport(dvsDone map[string]bool, numberOfDvs int) bool {
	// Count successfully imported dvs:
	done := utils.CountImportedDataVolumes(dvsDone)

	return done == numberOfDvs
}

func (r *ReconcileVirtualMachineImport) createDataVolume(provider provider.Provider, mapper provider.Mapper, instance *v2vv1.VirtualMachineImport, dv cdiv1.DataVolume, vmName types.NamespacedName) error {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	// Update condition to create VM:
	processingCond := conditions.NewProcessingCondition(string(v2vv1.CopyingDisks), "Copying virtual machine disks", corev1.ConditionTrue)
	err := r.upsertStatusConditions(instanceNamespacedName, processingCond)
	if err != nil {
		return err
	}
	// Update progress to copying disks:
	if err := r.updateProgress(instance, progressCopyingDisks); err != nil {
		return err
	}
	// Fetch VM:
	vmDef := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, vmDef)

	if err != nil {
		return err
	}

	// Set controller owner reference:
	if err := controllerutil.SetControllerReference(instance, &dv, r.scheme); err != nil {
		return err
	}

	// Set tracking label
	setTrackerLabel(dv.ObjectMeta, instance)

	err = r.client.Create(context.TODO(), &dv)
	if err != nil {
		message := fmt.Sprintf("Data volume %s/%s creation failed: %s", dv.Namespace, dv.Name, err)
		log.Error(err, message)
		return errors.New(message)
	}

	// Set VM as owner reference:
	if err := r.ownerreferencesmgr.AddOwnerReference(vmDef, &dv); err != nil {
		return err
	}

	// Emit event that DV import is in progress:
	r.recorder.Eventf(
		instance,
		corev1.EventTypeNormal,
		EventImportInProgress,
		"Import of Virtual Machine %s/%s disk %s in progress", vmName.Namespace, vmName.Name, dv.Name,
	)

	// Update datavolume in VM import CR status:
	if err = r.updateDVs(instanceNamespacedName, dv); err != nil {
		return err
	}

	// Update VM spec with imported disks:
	err = r.updateVMSpecDataVolumes(mapper, types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, dv)
	if err != nil {
		log.Error(err, "Cannot update VM with Data Volumes")
		return err
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) fetchSecret(vmImport *v2vv1.VirtualMachineImport) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretNamespace := vmImport.Namespace
	if vmImport.Spec.ProviderCredentialsSecret.Namespace != nil {
		secretNamespace = *vmImport.Spec.ProviderCredentialsSecret.Namespace
	}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: vmImport.Spec.ProviderCredentialsSecret.Name, Namespace: secretNamespace}, secret)
	return secret, err
}

func (r *ReconcileVirtualMachineImport) fetchResourceMapping(resourceMappingID *v2vv1.ObjectIdentifier, crNamespace string) (*v2vv1.ResourceMappingSpec, error) {
	if resourceMappingID == nil {
		return nil, nil
	}
	namespace := crNamespace
	if resourceMappingID.Namespace != nil {
		namespace = *resourceMappingID.Namespace
	}
	resourceMapping, err := r.resourceMappingsFinder.GetResourceMapping(types.NamespacedName{Name: resourceMappingID.Name, Namespace: namespace})
	if err != nil {
		return nil, err
	}
	return &resourceMapping.Spec, nil
}

func shouldInvoke(vmiStatus *v2vv1.VirtualMachineImportStatus) bool {
	validCondition := conditions.FindConditionOfType(vmiStatus.Conditions, v2vv1.Valid)
	rulesVerificationCondition := conditions.FindConditionOfType(vmiStatus.Conditions, v2vv1.MappingRulesVerified)

	return isIncomplete(validCondition) || isIncomplete(rulesVerificationCondition)
}

func isIncomplete(condition *v2vv1.VirtualMachineImportCondition) bool {
	return condition == nil || condition.Status != corev1.ConditionTrue
}

func (r *ReconcileVirtualMachineImport) createProvider(vmi *v2vv1.VirtualMachineImport) (provider.Provider, error) {
	config, err := r.ctrlConfigProvider.GetConfig()
	if err != nil {
		log.Error(err, "Cannot get controller config.")
	}

	if vmi.Spec.Source.Ovirt != nil && vmi.Spec.Source.Vmware != nil {
		return nil, fmt.Errorf("Invalid source. Must only include one source type.")
	}

	// The type of the provider is evaluated based on the source field from the CR
	if vmi.Spec.Source.Ovirt != nil {
		provider := ovirtprovider.NewOvirtProvider(vmi.ObjectMeta, vmi.TypeMeta, r.client, r.ocClient, r.factory, r.kvConfigProvider, config)
		return &provider, nil
	}
	if vmi.Spec.Source.Vmware != nil {
		provider := vmware.NewVmwareProvider(vmi.ObjectMeta, vmi.TypeMeta, r.client, r.ocClient, r.factory, config)
		return &provider, nil
	}

	return nil, fmt.Errorf("Invalid source type. Only Ovirt and Vmware type is supported")
}

func (r *ReconcileVirtualMachineImport) updateToRunning(vmName types.NamespacedName) error {
	var vm kubevirtv1.VirtualMachine
	err := r.client.Get(context.TODO(), vmName, &vm)
	if err != nil {
		return err
	}

	copy := vm.DeepCopy()
	running := true
	copy.Spec.Running = &running

	patch := client.MergeFrom(&vm)
	err = r.client.Patch(context.TODO(), copy, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) updateDVs(vmiName types.NamespacedName, dv cdiv1.DataVolume) error {
	var instance v2vv1.VirtualMachineImport
	err := r.apiReader.Get(context.TODO(), vmiName, &instance)
	if err != nil {
		return err
	}

	copy := instance.DeepCopy()
	dvFound := false
	for _, dvName := range copy.Status.DataVolumes {
		if dvName.Name == dv.Name {
			dvFound = true
			break
		}
	}
	// Patch the status only in case DV is not already part of the VMImport status:
	if !dvFound {
		copy.Status.DataVolumes = append(copy.Status.DataVolumes, v2vv1.DataVolumeItem{Name: dv.Name})
		err = r.client.Status().Update(context.TODO(), copy)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) updateTargetVMName(vmiName types.NamespacedName, vmName string) error {
	var instance v2vv1.VirtualMachineImport
	err := r.client.Get(context.TODO(), vmiName, &instance)
	if err != nil {
		return err
	}

	copy := instance.DeepCopy()
	copy.Status.TargetVMName = vmName

	patch := client.MergeFrom(&instance)
	err = r.client.Status().Patch(context.TODO(), copy, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) updateVMSpecDataVolumes(mapper provider.Mapper, vmName types.NamespacedName, dv cdiv1.DataVolume) error {
	var vm kubevirtv1.VirtualMachine
	err := r.client.Get(context.TODO(), vmName, &vm)
	if err != nil {
		return err
	}
	copy := vm.DeepCopy()
	mapper.MapDisk(copy, dv)

	patch := client.MergeFrom(&vm)
	err = r.client.Patch(context.TODO(), copy, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) upsertStatusConditions(vmiName types.NamespacedName, newConditions ...v2vv1.VirtualMachineImportCondition) error {
	var instance v2vv1.VirtualMachineImport
	err := r.apiReader.Get(context.TODO(), vmiName, &instance)
	if err != nil {
		return err
	}
	copy := instance.DeepCopy()
	for _, condition := range newConditions {
		conditions.UpsertCondition(copy, condition)
	}
	return r.client.Status().Update(context.TODO(), copy)
}

func (r *ReconcileVirtualMachineImport) storeSourceVMStatus(instance *v2vv1.VirtualMachineImport, vmStatus string) error {
	vmiCopy := instance.DeepCopy()
	if vmiCopy.Annotations == nil {
		vmiCopy.Annotations = make(map[string]string)
	}
	vmiCopy.Annotations[sourceVMInitialState] = vmStatus

	patch := client.MergeFrom(instance)
	return r.client.Patch(context.TODO(), vmiCopy, patch)
}

func (r *ReconcileVirtualMachineImport) updateProgress(instance *v2vv1.VirtualMachineImport, progress string) error {
	currentProgress, ok := instance.Annotations[AnnCurrentProgress]
	if !ok {
		currentProgress = "0"
	}
	currentProgressInt, err := strconv.Atoi(currentProgress)
	if err != nil {
		return err
	}
	newProgressInt, err := strconv.Atoi(progress)
	if err != nil {
		return err
	}
	if currentProgressInt < newProgressInt {
		vmiCopy := instance.DeepCopy()
		if vmiCopy.Annotations == nil {
			vmiCopy.Annotations = make(map[string]string)
		}
		vmiCopy.Annotations[AnnCurrentProgress] = progress

		patch := client.MergeFrom(instance)
		return r.client.Patch(context.TODO(), vmiCopy, patch)
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) afterSuccess(vmName types.NamespacedName, p provider.Provider, instance *v2vv1.VirtualMachineImport) error {
	metrics.ImportCounter.IncSuccessful()
	vmiName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	var errs []error
	err := p.CleanUp(false, instance, r.client)
	if err != nil {
		errs = append(errs, err)
	}

	e := r.ownerreferencesmgr.PurgeOwnerReferences(vmName)
	if len(e) > 0 {
		errs = append(errs, e...)
	}

	if len(errs) > 0 {
		return foldErrors(errs, "Import success", vmiName)
	}
	return nil
}

//TODO: use in proper places
func (r *ReconcileVirtualMachineImport) afterFailure(p provider.Provider, instance *v2vv1.VirtualMachineImport) error {
	metrics.ImportCounter.IncFailed()
	vmiName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	var errs []error
	err := p.CleanUp(true, instance, r.client)
	if err != nil {
		errs = append(errs, err)
	}
	err = r.restoreInitialVMState(vmiName, p)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return foldErrors(errs, "Import failure", vmiName)
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) restoreInitialVMState(vmiName types.NamespacedName, p provider.Provider) error {
	var instance v2vv1.VirtualMachineImport
	err := r.client.Get(context.TODO(), vmiName, &instance)
	if err != nil {
		return err
	}

	vmInitialState, found := instance.Annotations[sourceVMInitialState]
	if !found {
		return fmt.Errorf("VM didn't have initial state stored in '%s' annotation", sourceVMInitialState)
	}
	if vmInitialState == string(provider.VMStatusUp) {
		return p.StartVM()
	}
	// VM was already down
	return nil
}

func foldErrors(errs []error, prefix string, vmiName types.NamespacedName) error {
	message := ""
	for _, e := range errs {
		message = utils.WithMessage(message, e.Error())
	}
	return fmt.Errorf("%s clean-up for %v failed: %s", prefix, utils.ToLoggableResourceName(vmiName.Name, &vmiName.Namespace), message)
}

func shouldFailWith(conditions []v2vv1.VirtualMachineImportCondition) (bool, string) {
	var message string
	valid := true
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse {
			if condition.Message != nil {
				message = utils.WithMessage(message, *condition.Message)
			}
			valid = false
		}
	}
	return valid, message
}

func (r *ReconcileVirtualMachineImport) initProvider(instance *v2vv1.VirtualMachineImport, provider provider.Provider) (string, error) {
	// Fetch source provider secret
	sourceProviderSecretObj, err := r.fetchSecret(instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			message := "Secret not found"
			cerr := r.upsertValidationCondition(instance, v2vv1.SecretNotFound, message, err)
			if cerr != nil {
				return message, cerr
			}
		}
		return "Failed to read the secret", err
	}

	err = provider.Init(sourceProviderSecretObj, instance)
	if err != nil {
		message := "Source provider initialization failed"
		cerr := r.upsertValidationCondition(instance, v2vv1.UninitializedProvider, message, err)
		if cerr != nil {
			return message, cerr
		}
		return message, err
	}

	if !r.vmImportInProgress(instance) {
		err = provider.TestConnection()
		if err != nil {
			message := "Failed to connect to source provider"
			cerr := r.upsertValidationCondition(instance, v2vv1.UnreachableProvider, message, err)
			if cerr != nil {
				return message, cerr
			}
			return message, err
		}
	}

	return "", nil
}

func (r *ReconcileVirtualMachineImport) upsertValidationCondition(
	instance *v2vv1.VirtualMachineImport,
	reason v2vv1.ValidConditionReason,
	message string,
	err error) error {
	condition := newValidationCondition(reason, message+": "+err.Error())
	cerr := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, condition)
	if cerr != nil {
		return cerr
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) failNewImportProcess(instance *v2vv1.VirtualMachineImport, message string, failure error) error {
	msg := fmt.Sprintf("Failed to initialize the source provider (%s): %s", message, failure.Error())
	succeededCond := conditions.NewSucceededCondition(string(v2vv1.ValidationFailed), msg, corev1.ConditionFalse)
	err := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, succeededCond)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) fetchVM(instance *v2vv1.VirtualMachineImport, provider provider.Provider) error {
	logger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	if shouldInvoke(&instance.Status) {
		// Load source VM:
		err := provider.LoadVM(instance.Spec.Source)
		if err != nil {
			condition := newValidationCondition(v2vv1.SourceVMNotFound, "Failed to load source VM: "+err.Error())
			cerr := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, condition)
			if cerr != nil {
				return cerr
			}
			return err
		}
	} else {
		logger.Info("No need to fetch virtual machine - skipping")
	}

	// Load the external resource mapping
	resourceMapping, err := r.fetchResourceMapping(instance.Spec.ResourceMapping, instance.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			condition := newValidationCondition(v2vv1.ResourceMappingNotFound, "Resource mapping not found")
			cerr := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, condition)
			if cerr != nil {
				return cerr
			}
		}
		return err
	}

	// Prepare/merge the resourceMapping
	provider.PrepareResourceMapping(resourceMapping, instance.Spec.Source)

	return nil
}

func (r *ReconcileVirtualMachineImport) validate(instance *v2vv1.VirtualMachineImport, provider provider.Provider) (bool, error) {
	logger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	if shouldInvoke(&instance.Status) {
		conditions, err := provider.Validate()
		if err != nil {
			return true, err
		}
		err = r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, conditions...)
		if err != nil {
			return true, err
		}
		if valid, message := shouldFailWith(conditions); !valid {
			logger.Info("Import blocked. " + message)

			// Emit event vm import blocked:
			if vmName, err := provider.GetVMName(); err == nil {
				// This potentially flood events service, consider checking if event already occurred and don't emit it if it did,
				// if any performance implication occur.
				r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportBlocked, "Virtual Machine %s/%s import blocked: %s", instance.Namespace, vmName, message)
			}

			return false, nil
		}

		vmStatus, err := provider.GetVMStatus()
		if err != nil {
			return true, err
		}

		logger.Info("Storing source VM status", "status", vmStatus)
		err = r.storeSourceVMStatus(instance, string(vmStatus))
		if err != nil {
			return true, err
		}
	} else {
		logger.Info("VirtualMachineImport has already been validated positively. Skipping re-validation")
	}
	return true, nil
}

func (r *ReconcileVirtualMachineImport) templateMatchingFailed(errorMessage string, processingCond *v2vv1.VirtualMachineImportCondition, provider provider.Provider, instance *v2vv1.VirtualMachineImport) error {
	message := "Couldn't find matching template. Either change the virtual machine OS type or add a custom template configMap for it, and a common template if there is none. Refer to documentation for more details."
	succeededCond := conditions.NewSucceededCondition(string(v2vv1.VMTemplateMatchingFailed), message, corev1.ConditionFalse)

	processingCond.Status = corev1.ConditionFalse
	processingFailedReason := string(v2vv1.ProcessingFailed)
	processingCond.Reason = &processingFailedReason
	processingCond.Message = &errorMessage
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	if err := r.upsertStatusConditions(instanceNamespacedName, succeededCond, *processingCond); err != nil {
		return err
	}

	// Cleanup after failure
	if err := r.afterFailure(provider, instance); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) vmImportInProgress(instance *v2vv1.VirtualMachineImport) bool {
	_, found := instance.Annotations[AnnCurrentProgress]
	return found
}

func newValidationCondition(reason v2vv1.ValidConditionReason, message string) v2vv1.VirtualMachineImportCondition {
	return conditions.NewCondition(
		v2vv1.Valid,
		string(reason),
		message,
		v1.ConditionFalse,
	)
}

// disksImportProgress count progress as progressCopyingDisks + (allProgress / countOfDisks) * (progressForCopyDisk / 100)
// So for example for two disks of one done on 25% and second for 60% we go as -> 10 + (85 / 2) * 0.75 = 41.875%
func disksImportProgress(dvsImportProgress map[string]float64, dvCount float64) string {
	sumProgress := float64(0.0)
	for _, progress := range dvsImportProgress {
		sumProgress += progress
	}
	startProgress, _ := strconv.Atoi(progressCopyingDisks)
	disksAverageProgress := sumProgress / dvCount
	return fmt.Sprintf("%v", startProgress+int(disksAverageProgress*progressCopyDiskRange))
}
