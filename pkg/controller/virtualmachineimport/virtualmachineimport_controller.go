package virtualmachineimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"kubevirt.io/containerized-data-importer/pkg/common"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/kubevirt/vm-import-operator/pkg/providers/vmware"

	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"

	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"

	kvConfig "github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
	"github.com/kubevirt/vm-import-operator/pkg/metrics"
	"github.com/kubevirt/vm-import-operator/pkg/ownerreferences"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtprovider "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	templatev1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
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
	annAPIGroup          = "vmimport.v2v.kubevirt.io"
	sourceVMInitialState = annAPIGroup + "/source-vm-initial-state"
	// AnnCurrentProgress is annotations storing current progress of the vm import
	AnnCurrentProgress = annAPIGroup + "/progress"
	// AnnPropagate is annotation defining which values to propagate
	AnnPropagate = annAPIGroup + "/propagate-annotations"
	// AnnDVNetwork is propagated to the DataVolume importer pods.
	AnnDVNetwork = "k8s.v1.cni.cncf.io/networks"
	// AnnDVMultusNetwork is propagated to the DataVolume importer pods.
	AnnDVMultusNetwork = "v1.multus-cni.io/default-network"
	// TrackingLabel is a label used to track related entities.
	TrackingLabel = annAPIGroup + "/tracker"
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
	// EventWarmImportFailed is emmitted when a warm import attempt fails.
	EventWarmImportFailed = "WarmImportFailed"
	// EventVMNotFound is emitted when the target VM cannot be found, perhaps due to being deleted during an import.
	EventVMNotFound = "VMNotFound"

	SlowReQ = time.Second * 10
	FastReQ = time.Second * 2
	NoReQ   = time.Second * 0
)

var (
	log = logf.Log.WithName("controller_virtualmachineimport")
	// importPodRestartTolerance define how many restart of the import pod are tolerated before
	// we end the import as failed, by default it's 3.
	importPodRestartTolerance, _ = strconv.Atoi(os.Getenv("IMPORT_POD_RESTART_TOLERANCE"))
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

	controllerConfig, err := ctrlConfigProvider.GetConfig()
	if err != nil {
		log.Error(err, "Cannot get controller config.")
	}
	return &ReconcileVirtualMachineImport{client: client,
		apiReader:              reader,
		scheme:                 mgr.GetScheme(),
		resourceMappingsFinder: finder,
		ocClient:               tempClient,
		ownerreferencesmgr:     ownerreferencesmgr,
		factory:                factory,
		kvConfigProvider:       kvConfigProvider,
		ctrlConfigProvider:     ctrlConfigProvider,
		ctrlConfig:             controllerConfig,
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
		&source.Kind{Type: &corev1.Pod{}},
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
	ctrlConfig             ctrlConfig.ControllerConfig
	recorder               record.EventRecorder
	controller             controller.Controller
	apiReader              client.Reader
	filesystemOverhead     cdiv1.FilesystemOverhead
}

// Reconcile reads that state of the cluster for a VirtualMachineImport object and makes changes based on the state read
// and what is in the VirtualMachineImport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVirtualMachineImport) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling VirtualMachineImport")

	config, err := r.ctrlConfigProvider.GetConfig()
	if err != nil {
		log.Error(err, "Cannot get controller config.")
	} else {
		r.ctrlConfig = config
	}

	// Fetch the VirtualMachineImport instance
	instance := &v2vv1.VirtualMachineImport{}
	err = r.client.Get(context.TODO(), request.NamespacedName, instance)
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

	// Add finalizer to handle cancelled import
	if !r.vmImportInProgress(instance) {
		err := utils.AddFinalizer(instance, utils.CancelledImportFinalizer, r.client)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Handle deleted import
	if instance.DeletionTimestamp != nil {

		// We know that additional finalizers after this point are created when VM import is in progress
		// Therefore, if one of them fails and we return to Reconcile again ==> the code above will not add cancel finalizer again
		// Finalizers that do not rely on import state should be handled here

		// Cancelled import finalizer
		if utils.HasFinalizer(instance, utils.CancelledImportFinalizer) {
			err := utils.RemoveFinalizer(instance, utils.CancelledImportFinalizer, r.client)
			if err != nil {
				return reconcile.Result{}, err
			}

			metrics.ImportMetrics.IncCancelled()
			metrics.ImportMetrics.SaveDurationCancelled(calculateImportDuration(instance))
		}

		// If no more finalizers then return so resource can be deleted
		if len(instance.GetFinalizers()) == 0 {
			return reconcile.Result{}, nil
		}
	}

	r.filesystemOverhead, err = r.getCDIFilesystemOverhead()
	if err != nil {
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

	if instance.DeletionTimestamp != nil {
		if utils.HasFinalizer(instance, utils.RestoreVMStateFinalizer) {
			err := r.finalize(instance, provider)
			if err != nil {
				// requeue if locked
				return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
			}
		}
		if utils.HasFinalizer(instance, utils.CleanupSnapshotsFinalizer) && instance.Status.WarmImport.RootSnapshot != nil {
			_ = provider.RemoveVMSnapshot(*instance.Status.WarmImport.RootSnapshot, true)
			err = utils.RemoveFinalizer(instance, utils.CleanupSnapshotsFinalizer, r.client)
			if err != nil {
				reqLogger.Error(err, "Finalizing - failed to remove snapshot finalizer")
			}
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

	// don't stop the VM during a warm import unless it's time to finalize
	if !shouldWarmImport(provider, instance) || shouldFinalizeWarmImport(instance) {
		if _, ok := instance.Annotations[sourceVMInitialState]; !ok {
			vmStatus, err := provider.GetVMStatus()
			if err != nil {
				return reconcile.Result{}, err
			}

			log.Info("Storing source VM status", "status", vmStatus)
			err = r.storeSourceVMStatus(instance, string(vmStatus))
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		// Stop the VM
		if err = provider.StopVM(instance, r.client); err != nil {
			return reconcile.Result{}, err
		}
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

	// At this point the target VM should exist, so fail the import if it's not there.
	err = r.client.Get(context.TODO(), vmName, &kubevirtv1.VirtualMachine{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// end import in failure
			err := r.endVMNotFound(provider, instance)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	if shouldWarmImport(provider, instance) {
		if shouldFinalizeWarmImport(instance) {
			err = r.setupNextStage(provider, instance, mapper, vmName, true)
			if err != nil {
				return reconcile.Result{RequeueAfter: FastReQ}, err
			}
		} else {
			requeueAfter, err := r.warmImport(provider, instance, mapper, vmName, reqLogger)
			return reconcile.Result{RequeueAfter: requeueAfter}, err
		}
	}

	if shouldImportDisks(instance) {
		done, err := r.importDisks(provider, instance, mapper, vmName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !done {
			// if the datavolumes are waiting for first consumer, then
			// attempt to start the VM so that Kubevirt can schedule
			// it and allow the datavolumes to bind.
			waiting, err := r.dvsWaitingForFirstConsumer(instance, mapper, vmName)
			if err != nil {
				return reconcile.Result{RequeueAfter: FastReQ}, err
			}
			if waiting {
				log.Info("Waiting for data volumes to be bound.")
				err = r.setRunning(vmName, true)
				if err != nil {
					return reconcile.Result{RequeueAfter: FastReQ}, err
				}
			} else {
				// restore the original running state if the datavolumes are no longer waiting.
				err = r.setRunning(vmName, mapper.RunningState())
				if err != nil {
					return reconcile.Result{RequeueAfter: FastReQ}, err
				}
			}

			reqLogger.Info("Waiting for disks to be imported")
			return reconcile.Result{RequeueAfter: SlowReQ}, nil
		}
	}

	if shouldConvertGuest(provider, instance) {
		done, err := r.convertGuest(provider, instance, mapper, vmName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !done {
			reqLogger.Info("Waiting for guest to be converted")
			return reconcile.Result{RequeueAfter: SlowReQ}, nil
		}
	}

	if !conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady, v2vv1.VirtualMachineRunning) {
		if err := r.updateConditionsAfterSuccess(instance, "Virtual machine disks import done", v2vv1.VirtualMachineReady); err != nil {
			return reconcile.Result{}, err
		}
	}

	if shouldStartVM(instance) {
		var requeue bool
		if requeue, err = r.startVM(provider, instance, vmName); err != nil {
			return reconcile.Result{}, err
		}
		if requeue {
			// Requeue when vmi was not created yet or was not scheduled
			return reconcile.Result{RequeueAfter: time.Second * 15}, err
		}
	} else {
		// Update progress if all disks import done:
		if err := r.updateProgress(instance, progressDone); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.afterSuccess(vmName, provider, instance); err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("Virtual Machine imported successful without starting", "VM.name", vmName)
		// Emit event vm is successfully imported
		r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportSucceeded, "Virtual Machine %s/%s import successful", vmName.Namespace, vmName.Name)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVirtualMachineImport) getDataVolume(dvName types.NamespacedName) (*cdiv1.DataVolume, error) {
	dv := &cdiv1.DataVolume{}
	err := r.client.Get(context.TODO(), dvName, dv)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return dv, nil
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
func (r *ReconcileVirtualMachineImport) convertGuest(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName) (bool, error) {
	log := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	// find the vmspec
	vmSpec := &kubevirtv1.VirtualMachine{}
	err := r.client.Get(context.TODO(), vmName, vmSpec)
	if err != nil {
		return false, err
	}

	dataVolumes, err := mapper.MapDataVolumes(&vmName.Name, r.filesystemOverhead)
	if err != nil {
		return false, err
	}

	pod, err := provider.GetGuestConversionPod()
	if err != nil {
		return false, err
	}
	if pod == nil {
		log.Info("Creating conversion pod")
		pod, err = provider.LaunchGuestConversionPod(vmSpec, dataVolumes)
		if err != nil {
			return false, err
		}
		processingCond := conditions.NewProcessingCondition(string(v2vv1.ConvertingGuest), fmt.Sprintf("Running virt-v2v pod %s", pod.Name), corev1.ConditionTrue)
		err = r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, processingCond)
		if err != nil {
			return false, err
		}

		// Update progress to converting guest:
		if err = r.updateProgress(instance, progressConvertingGuest); err != nil {
			return false, err
		}
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		return true, nil
	} else if pod.Status.Phase == corev1.PodFailed {
		log.Info("Conversion pod failed.", "Pod.Name", pod.Name)
		err := r.endGuestConversionFailed(provider, instance, fmt.Sprintf("virt-v2v pod %s failed", pod.Name))
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (r *ReconcileVirtualMachineImport) importDisks(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName) (bool, error) {
	log := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)

	dvs, err := mapper.MapDataVolumes(&vmName.Name, r.filesystemOverhead)
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
				log.Info("Creating data volume", "DataVolume.Name", dv.Name, "VM.Name", vmName)
				if _, createErr := r.createDataVolume(provider, mapper, instance, &dv, vmName); createErr != nil {
					if err = r.endDiskImportFailed(provider, instance, foundDv, createErr.Error()); err != nil {
						return false, err
					}
					return false, createErr
				}
			} else {
				// If disk status is wrong, end the import as failed:
				log.Info("Disk status is incorrect", "DataVolume.Name", dv.Name, "VM.Name", vmName)
				if err = r.endDiskImportFailed(provider, instance, foundDv, "disk is in illegal status"); err != nil {
					return false, err
				}
			}
		} else if err == nil {
			instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
			// Set dataVolume as done, if it's in Succeeded state:
			if foundDv.Status.Phase == cdiv1.Succeeded {
				log.Info("Data volume import succeeded", "DataVolume.Name", foundDv.Name, "VM.Name", vmName)
				dvsDone[dvID] = true
			} else if foundDv.Status.Phase == cdiv1.Failed {
				log.Info("Data volume import failed", "DataVolume.Name", foundDv.Name, "VM.Name", vmName)
				if err = r.endDiskImportFailed(provider, instance, foundDv, "dv is in Failed Phase"); err != nil {
					return false, err
				}
			} else if foundDv.Status.Phase == cdiv1.Pending {
				// Update condition to pending the PVC bound:
				message := fmt.Sprintf("DataVolume %s is pending to bound.", foundDv.Name)
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

				// During ImportInProgress phase importer pod can be in crashloopbackoff, so we need
				// to check the state of the pod and fail the import:
				foundPod := &corev1.Pod{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: importerPodNameFromDv(dvID)}, foundPod)
				if err == nil {
					var terminationMessage string
					// Emit an event about why pod failed:
					if foundPod.Status.ContainerStatuses != nil &&
						foundPod.Status.ContainerStatuses[0].LastTerminationState.Terminated != nil &&
						foundPod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode > 0 {
						terminationMessage = foundPod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Message
						r.recorder.Eventf(instance, corev1.EventTypeWarning, EventPVCImportFailed, terminationMessage)
					}
					// End the import in case the pod keeps crashing:
					for _, cs := range foundPod.Status.ContainerStatuses {
						if cs.State.Waiting != nil && cs.State.Waiting.Reason == podCrashLoopBackOff && cs.RestartCount > int32(importPodRestartTolerance) {
							log.Info("CDI import pod failed.", "VM.Name", vmName)

							message := "pod CrashLoopBackoff restart exceeded"
							if terminationMessage != "" {
								message = fmt.Sprintf("%s (%s)", terminationMessage, message)
							}
							if err = r.endDiskImportFailed(provider, instance, foundDv, message); err != nil {
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

func (r *ReconcileVirtualMachineImport) dvsWaitingForFirstConsumer(instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName) (bool, error) {
	dvs, err := mapper.MapDataVolumes(&vmName.Name, r.filesystemOverhead)
	if err != nil {
		return false, err
	}

	for dvID, _ := range dvs {
		dvName := types.NamespacedName{Namespace: instance.Namespace, Name: dvID}
		dv, err := r.getDataVolume(dvName)
		if err != nil {
			return false, err
		}
		if dv == nil {
			continue
		}
		if dv.Status.Phase == cdiv1.WaitForFirstConsumer {
			return true, nil
		}
	}
	return false, nil
}

func (r *ReconcileVirtualMachineImport) fail(provider provider.Provider, instance *v2vv1.VirtualMachineImport, reason v2vv1.SucceededConditionReason, message string) error {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}

	// Update processing condition to failed:
	processingCond := conditions.NewProcessingCondition(string(v2vv1.ProcessingFailed), message, corev1.ConditionFalse)
	if err := r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
		return err
	}

	// Update succeed condition to failed:
	succeededCond := conditions.NewSucceededCondition(string(reason), message, corev1.ConditionFalse)
	if err := r.upsertStatusConditions(instanceNamespacedName, succeededCond); err != nil {
		return err
	}

	// Update progress to done.
	if err := r.updateProgress(instance, progressDone); err != nil {
		return err
	}

	// Cleanup
	if err := r.afterFailure(provider, instance); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) endVMNotFound(provider provider.Provider, instance *v2vv1.VirtualMachineImport) error {
	message := fmt.Sprintf("target VM %s not found", instance.Status.TargetVMName)

	// Update event:
	r.recorder.Event(instance, corev1.EventTypeWarning, EventVMNotFound, message)

	return r.fail(provider, instance, v2vv1.VMNotFound, message)
}

func (r *ReconcileVirtualMachineImport) endDiskImportFailed(provider provider.Provider, instance *v2vv1.VirtualMachineImport, dv *cdiv1.DataVolume, message string) error {
	// Update event:
	r.recorder.Event(instance, corev1.EventTypeWarning, EventDVCreationFailed, message)

	errorMessage := fmt.Sprintf("Error while importing disk image: %s. %s", dv.Name, message)
	return r.fail(provider, instance, v2vv1.DataVolumeCreationFailed, errorMessage)
}

func (r *ReconcileVirtualMachineImport) endGuestConversionFailed(provider provider.Provider, instance *v2vv1.VirtualMachineImport, message string) error {
	// Update event:
	r.recorder.Event(instance, corev1.EventTypeWarning, EventGuestConversionFailed, message)

	errorMessage := fmt.Sprintf("Error converting guests: %s", message)
	return r.fail(provider, instance, v2vv1.GuestConversionFailed, errorMessage)
}

func (r *ReconcileVirtualMachineImport) endWarmImportFailed(provider provider.Provider, instance *v2vv1.VirtualMachineImport, message string) error {
	// Update event:
	r.recorder.Event(instance, corev1.EventTypeWarning, EventWarmImportFailed, message)

	errorMessage := fmt.Sprintf("Error while attempting warm import: %s", message)
	return r.fail(provider, instance, v2vv1.WarmImportFailed, errorMessage)
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
	config, cfgErr := r.ctrlConfigProvider.GetConfig()
	if cfgErr != nil {
		log.Error(cfgErr, "Cannot get controller config.")
	}
	kvConfig, cfgErr := r.kvConfigProvider.GetConfig()
	if cfgErr != nil {
		log.Error(cfgErr, "Cannot get KubeVirt cluster config.")
	}
	if err != nil {
		reqLogger.Info("No matching template was found for the virtual machine.")
		if !config.ImportWithoutTemplateEnabled() && !kvConfig.ImportWithoutTemplateEnabled() {
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
			if !config.ImportWithoutTemplateEnabled() && !kvConfig.ImportWithoutTemplateEnabled() {
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
	reqLogger.Info("Mapping virtual machine resources.", "VM.Name", targetVMName)
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
	if createErr := r.client.Create(context.TODO(), vmSpec); createErr != nil && !k8serrors.IsAlreadyExists(createErr) {
		vmJSON, _ := json.Marshal(vmSpec)
		reqLogger.Info("VM struct", "VM spec", string(vmJSON))
		message := fmt.Sprintf("Error while creating virtual machine %s/%s: %s", vmSpec.Namespace, vmSpec.Name, createErr)
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
		return "", createErr
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

func setDVNetworkAnnotations(instance *v2vv1.VirtualMachineImport, dv *cdiv1.DataVolume) {
	annotations := instance.GetAnnotations()
	dvAnnotations := dv.GetAnnotations()
	if dvAnnotations == nil {
		dvAnnotations = map[string]string{}
		dv.SetAnnotations(dvAnnotations)
	}
	dvNetwork := annotations[AnnDVNetwork]
	if dvNetwork != "" {
		dvAnnotations[AnnDVNetwork] = annotations[AnnDVNetwork]
	}
	dvMultusNetwork := annotations[AnnDVMultusNetwork]
	if dvMultusNetwork != "" {
		dvAnnotations[AnnDVMultusNetwork] = annotations[AnnDVMultusNetwork]
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
func (r *ReconcileVirtualMachineImport) startVM(provider provider.Provider, instance *v2vv1.VirtualMachineImport, vmName types.NamespacedName) (bool, error) {
	log := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	vmi := &kubevirtv1.VirtualMachineInstance{}
	err := r.client.Get(context.TODO(), vmName, vmi)
	vmIdentifier := utils.ToLoggableResourceName(vmName.Name, &vmName.Namespace)
	log.Info("startVM method", "VM.Name", vmName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("Updating progress while starting vm", "VM.Name", vmName)
			if err = r.updateProgress(instance, progressStartVM); err != nil {
				return false, err
			}
			log.Info("Starting a vm", "VM.Name", vmName)
			if err = r.setRunning(vmName, true); err != nil {
				// Emit event vm failed to start:
				r.recorder.Eventf(instance, corev1.EventTypeWarning, EventVMStartFailed, "Virtual Machine %s failed to start: %s", vmIdentifier, err)
				return false, err
			}
			return true, nil
		}
		return false, err
	}

	log.Info("VMI available", "VM.Name", vmName)
	if vmi.Status.Phase == kubevirtv1.Running || vmi.Status.Phase == kubevirtv1.Scheduled {
		log.Info("The vm started", "VM.Name", vmName)
		// Emit event vm is successfully imported and started:
		r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportSucceeded, "Virtual Machine %s imported and started", vmIdentifier)
		if err = r.updateConditionsAfterSuccess(instance, "Virtual machine running", v2vv1.VirtualMachineRunning); err != nil {
			return false, err
		}
		if err = r.updateProgress(instance, progressDone); err != nil {
			return false, err
		}
		if err = r.afterSuccess(vmName, provider, instance); err != nil {
			return false, err
		}
	} else {
		return true, nil
	}
	return false, nil
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
	return conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady) &&
		((instance.Spec.StartVM != nil && *instance.Spec.StartVM) ||
			(instance.Spec.StartVM == nil &&
				instance.Spec.Warm &&
				instance.Annotations[sourceVMInitialState] == string(provider.VMStatusUp)))
}

func shouldConvertGuest(provider provider.Provider, instance *v2vv1.VirtualMachineImport) bool {
	return provider.NeedsGuestConversion() && !conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1.VirtualMachineReady, v2vv1.VirtualMachineRunning)
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

func (r *ReconcileVirtualMachineImport) createDataVolume(provider provider.Provider, mapper provider.Mapper, instance *v2vv1.VirtualMachineImport, dv *cdiv1.DataVolume, vmName types.NamespacedName) (*cdiv1.DataVolume, error) {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	// Update condition to create VM:
	processingCond := conditions.NewProcessingCondition(string(v2vv1.CopyingDisks), "Copying virtual machine disks", corev1.ConditionTrue)
	err := r.upsertStatusConditions(instanceNamespacedName, processingCond)
	if err != nil {
		return nil, err
	}
	// Update progress to copying disks:
	if err := r.updateProgress(instance, progressCopyingDisks); err != nil {
		return nil, err
	}
	// Fetch VM:
	vmDef := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, vmDef)
	if err != nil {
		return nil, err
	}

	// Set controller owner reference:
	if err := controllerutil.SetControllerReference(instance, dv, r.scheme); err != nil {
		return nil, err
	}

	// Set tracking label
	setTrackerLabel(dv.ObjectMeta, instance)

	// Set transfer network annotations
	setDVNetworkAnnotations(instance, dv)

	err = r.client.Create(context.TODO(), dv)
	if err != nil {
		message := fmt.Sprintf("Data volume %s/%s creation failed: %s", dv.Namespace, dv.Name, err)
		log.Error(err, message)
		return nil, errors.New(message)
	}

	// Set VM as owner reference:
	if err := r.ownerreferencesmgr.AddOwnerReference(vmDef, dv); err != nil {
		return nil, err
	}

	// Emit event that DV import is in progress:
	r.recorder.Eventf(
		instance,
		corev1.EventTypeNormal,
		EventImportInProgress,
		"Import of Virtual Machine %s/%s disk %s in progress", vmName.Namespace, vmName.Name, dv.Name,
	)

	// Update datavolume in VM import CR status:
	if err = r.updateDVs(instanceNamespacedName, *dv); err != nil {
		return nil, err
	}

	// Update VM spec with imported disks:
	err = r.updateVMSpecDataVolumes(mapper, types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, *dv)
	if err != nil {
		log.Error(err, "Cannot update VM with Data Volumes")
		return nil, err
	}

	return dv, nil
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
	if vmi.Spec.Source.Ovirt != nil && vmi.Spec.Source.Vmware != nil {
		return nil, fmt.Errorf("Invalid source. Must only include one source type.")
	}

	// The type of the provider is evaluated based on the source field from the CR
	if vmi.Spec.Source.Ovirt != nil {
		provider := ovirtprovider.NewOvirtProvider(vmi.ObjectMeta, vmi.TypeMeta, r.client, r.ocClient, r.factory, r.kvConfigProvider, r.ctrlConfig)
		return &provider, nil
	}
	if vmi.Spec.Source.Vmware != nil {
		provider := vmware.NewVmwareProvider(vmi.ObjectMeta, vmi.TypeMeta, r.client, r.ocClient, r.factory, r.ctrlConfig)
		return &provider, nil
	}

	return nil, fmt.Errorf("Invalid source type. Only Ovirt and Vmware type is supported")
}

func (r *ReconcileVirtualMachineImport) setRunning(vmName types.NamespacedName, running bool) error {
	var vm kubevirtv1.VirtualMachine
	err := r.client.Get(context.TODO(), vmName, &vm)
	if err != nil {
		return err
	}

	copy := vm.DeepCopy()
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

	r.removeFinalizer(utils.CancelledImportFinalizer, instance)

	metrics.ImportMetrics.IncSuccessful()
	metrics.ImportMetrics.SaveDurationSuccessful(calculateImportDuration(instance))

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

	r.removeFinalizer(utils.CancelledImportFinalizer, instance)

	metrics.ImportMetrics.IncFailed()
	metrics.ImportMetrics.SaveDurationFailed(calculateImportDuration(instance))

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

func (r *ReconcileVirtualMachineImport) getCDIFilesystemOverhead() (cdiv1.FilesystemOverhead, error) {
	filesystemOverhead := cdiv1.FilesystemOverhead{
		Global:       common.DefaultGlobalOverhead,
		StorageClass: make(map[string]cdiv1.Percent),
	}

	cdiConfig := &cdiv1.CDIConfig{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "config", Namespace: ""}, cdiConfig)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return filesystemOverhead, nil
		}
		return filesystemOverhead, err
	}

	if cdiConfig.Spec.FilesystemOverhead != nil {
		return *cdiConfig.Spec.FilesystemOverhead, nil
	}

	return filesystemOverhead, nil
}

func validateName(instance *v2vv1.VirtualMachineImport, sourceName string) error {
	var message string
	var name string
	if instance.Spec.TargetVMName != nil {
		message = "`targetVmName` is not a valid k8s name: %v"
		name = *instance.Spec.TargetVMName
	} else {
		message = "Source VM name is not a valid k8s name: %v"
		name = sourceName
	}

	errs := k8svalidation.IsDNS1123Label(name)
	if len(errs) != 0 {
		var errString string
		for _, e := range errs {
			errString = utils.WithMessage(errString, e)
		}
		return fmt.Errorf(message, errString)
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) validateUniqueness(instance *v2vv1.VirtualMachineImport, sourceName string) (bool, error) {
	var name string
	if instance.Spec.TargetVMName != nil {
		name = *instance.Spec.TargetVMName
	} else {
		name = sourceName
	}

	namespacedName := types.NamespacedName{Namespace: instance.Namespace, Name: name}
	err := r.client.Get(context.TODO(), namespacedName, &kubevirtv1.VirtualMachine{})
	if err != nil && k8serrors.IsNotFound(err) {
		return true, nil
	}

	return false, err
}

func (r *ReconcileVirtualMachineImport) validate(instance *v2vv1.VirtualMachineImport, provider provider.Provider) (bool, error) {
	logger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	if shouldInvoke(&instance.Status) {

		vmName, err := provider.GetVMName()
		if err != nil {
			return false, err
		}

		err = validateName(instance, vmName)
		if err != nil {
			invalidNameCond := conditions.NewCondition(v2vv1.Valid, string(v2vv1.InvalidTargetVMName), err.Error(), corev1.ConditionFalse)
			err := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, invalidNameCond)
			return false, err
		}

		unique, err := r.validateUniqueness(instance, vmName)
		if err != nil {
			return false, err
		}
		if !unique {
			duplicateNameCond := conditions.NewCondition(v2vv1.Valid, string(v2vv1.DuplicateTargetVMName), "Virtual machine already exists in target namespace", corev1.ConditionFalse)
			err := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, duplicateNameCond)
			return false, err
		}

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
			// This potentially flood events service, consider checking if event already occurred and don't emit it if it did,
			// if any performance implication occur.
			r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportBlocked, "Virtual Machine %s/%s import blocked: %s", instance.Namespace, vmName, message)
			return false, nil
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

func (r *ReconcileVirtualMachineImport) removeFinalizer(finalizer string, instance *v2vv1.VirtualMachineImport) {
	err := utils.RemoveFinalizer(instance, finalizer, r.client)
	if err != nil {
		reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
		reqLogger.Error(err, "Failed to remove finalizer "+finalizer)

	}
}

func calculateImportDuration(instance *v2vv1.VirtualMachineImport) float64 {
	var endTime, startTime time.Time
	if instance.GetDeletionTimestamp() == nil {
		endTime = time.Now().UTC()
	} else {
		endTime = instance.GetDeletionTimestamp().Time.UTC()
	}

	startTime = instance.GetCreationTimestamp().Time.UTC()

	return endTime.Sub(startTime).Seconds()
}
