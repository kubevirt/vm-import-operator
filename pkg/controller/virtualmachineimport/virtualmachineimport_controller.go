package virtualmachineimport

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kubevirt/vm-import-operator/pkg/config"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
	"github.com/kubevirt/vm-import-operator/pkg/ownerreferences"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtprovider "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	templatev1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	sourceVMInitialState = "vmimport.v2v.kubevirt.io/source-vm-initial-state"
	// AnnCurrentProgress is annotations storing current progress of the vm import
	AnnCurrentProgress = "vmimport.v2v.kubevirt.io/progress"
	// constants
	progressStart                     = "0"
	progressCreatingVM                = "30"
	progressCopyingDisks              = "40"
	progressStartVM                   = "90"
	progressDone                      = "100"
	progressForCopyDisk               = 40
	requeueAfterValidationFailureTime = 5 * time.Second

	// EventImportSucceeded is emitted
	EventImportSucceeded = "ImportSucceeded"
	// EventImportBlocked is emitted
	EventImportBlocked = "ImportBlocked"
	// EventVMStartFailed is emitted
	EventVMStartFailed = "VMStartFailed"
	// EventVMCreationFailed is emitted
	EventVMCreationFailed = "VMCreationFailed"
	// EventDVCreationFailed is emitted
	EventDVCreationFailed = "DVCreationFailed"
)

var (
	log = logf.Log.WithName("controller_virtualmachineimport")
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new VirtualMachineImport Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, configProvider config.KubeVirtConfigProvider) error {
	return add(mgr, newReconciler(mgr, configProvider))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kvConfigProvider config.KubeVirtConfigProvider) reconcile.Reconciler {
	tempClient, err := templatev1.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "Unable to get OC client")
		panic("Controller cannot operate without OC client")
	}
	client := mgr.GetClient()
	finder := mappings.NewResourceMappingsFinder(client)
	ownerreferencesmgr := ownerreferences.NewOwnerReferenceManager(client)
	factory := ovirtprovider.NewSourceClientFactory()
	return &ReconcileVirtualMachineImport{client: client,
		scheme:                 mgr.GetScheme(),
		resourceMappingsFinder: finder,
		ocClient:               tempClient,
		ownerreferencesmgr:     ownerreferencesmgr,
		factory:                factory,
		kvConfigProvider:       kvConfigProvider,
		recorder:               mgr.GetEventRecorderFor("virtualmachineimport-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("virtualmachineimport-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VirtualMachineImport
	err = c.Watch(
		&source.Kind{Type: &v2vv1alpha1.VirtualMachineImport{}},
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
			OwnerType:    &v2vv1alpha1.VirtualMachineImport{},
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
			OwnerType:    &v2vv1alpha1.VirtualMachineImport{},
		},
	)
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
	kvConfigProvider       config.KubeVirtConfigProvider
	recorder               record.EventRecorder
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
	instance := &v2vv1alpha1.VirtualMachineImport{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
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
	err = r.initProvider(instance, provider)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer provider.Close()

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
	if err = provider.StopVM(); err != nil {
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
	}

	// Import disks:
	err = r.importDisks(provider, instance, mapper, vmName, request.NamespacedName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err = r.startVM(provider, instance, vmName); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVirtualMachineImport) importDisks(provider provider.Provider, instance *v2vv1alpha1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName, vmiName types.NamespacedName) error {
	dvs, err := mapper.MapDataVolumes()
	if err != nil {
		return err
	}
	dvsDone := make(map[string]bool)
	for dvID := range dvs {
		foundDv := &cdiv1.DataVolume{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: dvID}, foundDv)
		if err != nil && errors.IsNotFound(err) {
			if err = r.createDataVolumes(provider, mapper, instance, dvs, vmName); err != nil {
				return err
			}
		} else if err == nil {
			// Set dataVolume as done, if it's in Succeeded state:
			if foundDv.Status.Phase == cdiv1.Succeeded {
				dvsDone[dvID] = true
				if err = r.manageDataVolumeState(instance, dvsDone, len(dvs)); err != nil {
					return err
				}

				// Cleanup if user don't want to start the VM
				if instance.Spec.StartVM == nil || !*instance.Spec.StartVM {
					if err := r.updateProgress(instance, progressDone); err != nil {
						return err
					}
					if err := r.afterSuccess(vmName, vmiName, provider); err != nil {
						return err
					}

					// Emit event vm is successfully imported
					r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportSucceeded, "Virtual Machine %s/%s import successful", vmName.Namespace, vmName.Name)
				}
			}
		} else {
			return err
		}
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) createVM(provider provider.Provider, instance *v2vv1alpha1.VirtualMachineImport, mapper provider.Mapper) (string, error) {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)

	// Resolve VM Name
	targetVMName := mapper.ResolveVMName(instance.Spec.TargetVMName)

	// Define VM spec
	template, err := provider.FindTemplate()
	var spec *kubevirtv1.VirtualMachine
	if err != nil {
		reqLogger.Info("No matching template was found for the virtual machine. Using empty VM definition")
		spec = mapper.CreateEmptyVM(targetVMName)
	} else {
		reqLogger.Info("A template was found for creating the virtual machine", "Template.Name", template.ObjectMeta.Name)
		spec, err = provider.ProcessTemplate(template, targetVMName, instance.Namespace)
		if err != nil {
			reqLogger.Info("Failed to process the template. Using empty VM definition. Error: " + err.Error())
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

	// Update progress:
	if err = r.updateProgress(instance, progressStart); err != nil {
		return "", err
	}

	// Set VirtualMachineImport instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, vmSpec, r.scheme); err != nil {
		return "", err
	}

	// Update condition to creating VM:
	processingCond := conditions.NewProcessingCondition(string(v2vv1alpha1.CreatingTargetVM), "Creating virtual machine", corev1.ConditionTrue)
	if err = r.upsertStatusConditions(instanceNamespacedName, processingCond); err != nil {
		return "", err
	}

	// Create kubevirt VM from source VM:
	reqLogger.Info("Creating a new VM", "VM.Namespace", vmSpec.Namespace, "VM.Name", vmSpec.Name)
	if err = r.client.Create(context.TODO(), vmSpec); err != nil && !errors.IsAlreadyExists(err) {
		message := fmt.Sprintf("Error while creating virtual machine %s/%s: %s", vmSpec.Namespace, vmSpec.Name, err)
		// Update event:
		r.recorder.Event(instance, corev1.EventTypeWarning, EventVMCreationFailed, message)

		// Update condition to failed state:
		succeededCond := conditions.NewSucceededCondition(string(v2vv1alpha1.VMCreationFailed), message, corev1.ConditionFalse)
		processingCond.Status = corev1.ConditionFalse
		processingFailedReason := string(v2vv1alpha1.ProcessingFailed)
		processingCond.Reason = &processingFailedReason
		if err = r.upsertStatusConditions(instanceNamespacedName, succeededCond, processingCond); err != nil {
			return "", err
		}

		// Cleanup after failure
		if err = r.afterFailure(instanceNamespacedName, provider); err != nil {
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

// startVM start the VM if was requested to be started and VM disks are imported and ready:
func (r *ReconcileVirtualMachineImport) startVM(provider provider.Provider, instance *v2vv1alpha1.VirtualMachineImport, vmName types.NamespacedName) error {
	if shouldStartVM(instance) {
		vmi := &kubevirtv1.VirtualMachineInstance{}
		err := r.client.Get(context.TODO(), vmName, vmi)
		if err != nil && errors.IsNotFound(err) {
			if err = r.updateProgress(instance, progressStartVM); err != nil {
				return err
			}
			if err = r.updateToRunning(vmName); err != nil {
				// Emit event vm failed to start:
				r.recorder.Eventf(instance, corev1.EventTypeWarning, EventVMStartFailed, "Virtual Machine %s/%s failed to start: %s", vmName.Namespace, vmName.Name, err)
				return err
			}
		} else if err == nil {
			if vmi.Status.Phase == kubevirtv1.Running || vmi.Status.Phase == kubevirtv1.Scheduled {
				// Emit event vm is successfully imported and started:
				r.recorder.Eventf(instance, corev1.EventTypeNormal, EventImportSucceeded, "Virtual Machine %s/%s imported and started", vmName.Namespace, vmName.Name)

				if err = r.updateConditionsAfterSuccess(instance, "Virtual machine running", v2vv1alpha1.VirtualMachineRunning); err != nil {
					return err
				}
				if err = r.updateProgress(instance, progressDone); err != nil {
					return err
				}
				instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
				if err = r.afterSuccess(vmName, instanceNamespacedName, provider); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) updateConditionsAfterSuccess(instance *v2vv1alpha1.VirtualMachineImport, message string, reason v2vv1alpha1.SucceededConditionReason) error {
	succeededCond := conditions.NewSucceededCondition(string(reason), message, corev1.ConditionTrue)
	conds := []v2vv1alpha1.VirtualMachineImportCondition{succeededCond}

	processingCond := conditions.FindConditionOfType(instance.Status.Conditions, v2vv1alpha1.Processing)
	if processingCond != nil {
		processingCond.Status = corev1.ConditionFalse
		processingCompletedReason := string(v2vv1alpha1.ProcessingCompleted)
		processingCond.Reason = &processingCompletedReason
		conds = append(conds, *processingCond)
	}

	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	return r.upsertStatusConditions(instanceNamespacedName, conds...)
}

func shouldStartVM(instance *v2vv1alpha1.VirtualMachineImport) bool {
	return instance.Spec.StartVM != nil && *instance.Spec.StartVM && conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1alpha1.VirtualMachineReady)
}

// manageDataVolumeState update current state according to progress of import
func (r *ReconcileVirtualMachineImport) manageDataVolumeState(instance *v2vv1alpha1.VirtualMachineImport, dvsDone map[string]bool, numberOfDvs int) error {
	// Count successfully imported dvs:
	done := utils.CountImportedDataVolumes(dvsDone)

	// If all DVs was imported - update state
	allDone := done == numberOfDvs
	if allDone && !conditions.HasSucceededConditionOfReason(instance.Status.Conditions, v2vv1alpha1.VirtualMachineReady, v2vv1alpha1.VirtualMachineRunning) {
		if err := r.updateConditionsAfterSuccess(instance, "Virtual machine disks import done", v2vv1alpha1.VirtualMachineReady); err != nil {
			return err
		}
	}

	// Update progress, progress of CopyingDisks starts at 40% and we update the proccess,
	// based on number of disks. Each disk updates state as 40/numberOfDisks. So we end at 80%.
	if done > 0 {
		progressCopyingDisksInt, _ := strconv.Atoi(progressCopyingDisks)
		if err := r.updateProgress(instance, strconv.FormatInt(int64(progressCopyingDisksInt+(progressForCopyDisk/done)), 10)); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) createDataVolumes(provider provider.Provider, mapper provider.Mapper, instance *v2vv1alpha1.VirtualMachineImport, dvs map[string]cdiv1.DataVolume, vmName types.NamespacedName) error {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	// Update condition to create VM:
	processingCond := conditions.NewProcessingCondition(string(v2vv1alpha1.CopyingDisks), "Copying virtual machine disks", corev1.ConditionTrue)
	err := r.upsertStatusConditions(instanceNamespacedName, processingCond)
	if err != nil {
		return err
	}
	// Update progress to copying disks:
	if err = r.updateProgress(instance, progressCopyingDisks); err != nil {
		return err
	}
	// Fetch VM:
	vmDef := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, vmDef)

	if err != nil {
		return err
	}
	// Create DVs:
	for _, dv := range dvs {
		// Set controller owner reference:
		if err := controllerutil.SetControllerReference(instance, &dv, r.scheme); err != nil {
			return err
		}

		err = r.client.Create(context.TODO(), &dv)
		if err != nil {
			message := fmt.Sprintf("Data volume %s/%s creation failed: %s", dv.Namespace, dv.Name, err)
			// Update event:
			r.recorder.Event(instance, corev1.EventTypeWarning, EventDVCreationFailed, message)

			// Update condition to failed:
			succeededCond := conditions.NewSucceededCondition(string(v2vv1alpha1.DataVolumeCreationFailed), message, corev1.ConditionFalse)
			processingCond.Status = corev1.ConditionFalse
			processingFailedReason := string(v2vv1alpha1.ProcessingFailed)
			processingCond.Reason = &processingFailedReason
			if err = r.upsertStatusConditions(instanceNamespacedName, processingCond, succeededCond); err != nil {
				return err
			}

			// Cleanup
			if err = r.afterFailure(instanceNamespacedName, provider); err != nil {
				return err
			}
			return err
		}

		// Set VM as owner reference:
		if err := r.ownerreferencesmgr.AddOwnerReference(vmDef, &dv); err != nil {
			return err
		}
	}

	// Update datavolume in VM import CR status:
	if err = r.updateDVs(instanceNamespacedName, dvs); err != nil {
		return err
	}

	// Update VM spec with imported disks:
	err = r.updateVMSpecDataVolumes(mapper, types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, dvs)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) fetchSecret(vmImport *v2vv1alpha1.VirtualMachineImport) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretNamespace := vmImport.Namespace
	if vmImport.Spec.ProviderCredentialsSecret.Namespace != nil {
		secretNamespace = *vmImport.Spec.ProviderCredentialsSecret.Namespace
	}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: vmImport.Spec.ProviderCredentialsSecret.Name, Namespace: secretNamespace}, secret)
	return secret, err
}

func (r *ReconcileVirtualMachineImport) fetchResourceMapping(resourceMappingID *v2vv1alpha1.ObjectIdentifier, crNamespace string) (*v2vv1alpha1.ResourceMappingSpec, error) {
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

func shouldInvoke(vmiStatus *v2vv1alpha1.VirtualMachineImportStatus) bool {
	validCondition := conditions.FindConditionOfType(vmiStatus.Conditions, v2vv1alpha1.Valid)
	rulesVerificationCondition := conditions.FindConditionOfType(vmiStatus.Conditions, v2vv1alpha1.MappingRulesVerified)

	return isIncomplete(validCondition) || isIncomplete(rulesVerificationCondition)
}

func isIncomplete(condition *v2vv1alpha1.VirtualMachineImportCondition) bool {
	return condition == nil || condition.Status != corev1.ConditionTrue
}

func (r *ReconcileVirtualMachineImport) createProvider(vmi *v2vv1alpha1.VirtualMachineImport) (provider.Provider, error) {
	// The type of the provider is evaluated based on the source field from the CR
	if vmi.Spec.Source.Ovirt != nil {
		provider := ovirtprovider.NewOvirtProvider(vmi.ObjectMeta, vmi.TypeMeta, r.client, r.ocClient, r.factory, r.kvConfigProvider)
		return &provider, nil
	}

	return nil, fmt.Errorf("Invalid source type. only Ovirt type is supported")
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

func (r *ReconcileVirtualMachineImport) updateDVs(vmiName types.NamespacedName, dvs map[string]cdiv1.DataVolume) error {
	var instance v2vv1alpha1.VirtualMachineImport
	err := r.client.Get(context.TODO(), vmiName, &instance)
	if err != nil {
		return err
	}

	copy := instance.DeepCopy()
	for dvName := range dvs {
		copy.Status.DataVolumes = append(copy.Status.DataVolumes, v2vv1alpha1.DataVolumeItem{Name: dvName})
	}

	patch := client.MergeFrom(&instance)
	err = r.client.Status().Patch(context.TODO(), copy, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) updateTargetVMName(vmiName types.NamespacedName, vmName string) error {
	var instance v2vv1alpha1.VirtualMachineImport
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

func (r *ReconcileVirtualMachineImport) updateVMSpecDataVolumes(mapper provider.Mapper, vmName types.NamespacedName, dvs map[string]cdiv1.DataVolume) error {
	var vm kubevirtv1.VirtualMachine
	err := r.client.Get(context.TODO(), vmName, &vm)
	if err != nil {
		return err
	}
	copy := vm.DeepCopy()
	mapper.MapDisks(copy, dvs)

	patch := client.MergeFrom(&vm)
	err = r.client.Patch(context.TODO(), copy, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) upsertStatusConditions(vmiName types.NamespacedName, newConditions ...v2vv1alpha1.VirtualMachineImportCondition) error {
	var instance v2vv1alpha1.VirtualMachineImport
	err := r.client.Get(context.TODO(), vmiName, &instance)
	if err != nil {
		return err
	}

	copy := instance.DeepCopy()
	for _, condition := range newConditions {
		conditions.UpsertCondition(copy, condition)
	}

	patch := client.MergeFrom(&instance)
	err = r.client.Status().Patch(context.TODO(), copy, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) storeSourceVMStatus(instance *v2vv1alpha1.VirtualMachineImport, vmStatus string) error {
	vmiCopy := instance.DeepCopy()
	if vmiCopy.Annotations == nil {
		vmiCopy.Annotations = make(map[string]string)
	}
	vmiCopy.Annotations[sourceVMInitialState] = vmStatus

	patch := client.MergeFrom(instance)
	return r.client.Patch(context.TODO(), vmiCopy, patch)
}

func (r *ReconcileVirtualMachineImport) updateProgress(instance *v2vv1alpha1.VirtualMachineImport, progress string) error {
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

func (r *ReconcileVirtualMachineImport) afterSuccess(vmName types.NamespacedName, vmiName types.NamespacedName, p provider.Provider) error {
	var errs []error
	err := p.CleanUp()
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
func (r *ReconcileVirtualMachineImport) afterFailure(vmiName types.NamespacedName, p provider.Provider) error {
	var errs []error
	err := p.CleanUp()
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
	var instance v2vv1alpha1.VirtualMachineImport
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

func shouldFailWith(conditions []v2vv1alpha1.VirtualMachineImportCondition) (bool, string) {
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

func (r *ReconcileVirtualMachineImport) initProvider(instance *v2vv1alpha1.VirtualMachineImport, provider provider.Provider) error {
	// Fetch source provider secret
	sourceProviderSecretObj, err := r.fetchSecret(instance)
	if err != nil {
		if errors.IsNotFound(err) {
			condition := newValidationCondition(v2vv1alpha1.SecretNotFound, "Secret not found")
			cerr := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, condition)
			if cerr != nil {
				return cerr
			}
		}
		return err
	}

	err = provider.Init(sourceProviderSecretObj, instance)
	if err != nil {
		condition := newValidationCondition(v2vv1alpha1.UninitializedProvider, "Failed to initialize the source provider")
		cerr := r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, condition)
		if cerr != nil {
			return cerr
		}
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) fetchVM(instance *v2vv1alpha1.VirtualMachineImport, provider provider.Provider) error {
	logger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	if shouldInvoke(&instance.Status) {
		// Load source VM:
		err := provider.LoadVM(instance.Spec.Source)
		if err != nil {
			condition := newValidationCondition(v2vv1alpha1.SourceVMNotFound, "Failed to load source VM: "+err.Error())
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
		if errors.IsNotFound(err) {
			condition := newValidationCondition(v2vv1alpha1.ResourceMappingNotFound, "Resource mapping not found")
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

func (r *ReconcileVirtualMachineImport) validate(instance *v2vv1alpha1.VirtualMachineImport, provider provider.Provider) (bool, error) {
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
				// This potentially flood events service, consider checking if event already occured and don't emit it if it did,
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

func newValidationCondition(reason v2vv1alpha1.ValidConditionReason, message string) v2vv1alpha1.VirtualMachineImportCondition {
	return conditions.NewCondition(
		v2vv1alpha1.Valid,
		string(reason),
		message,
		v1.ConditionFalse,
	)
}
