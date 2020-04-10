package virtualmachineimport

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtprovider "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	templatev1 "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	tempClient, err := templatev1.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "Unable to get OC client")
		panic("Controller cannot operate without OC client")
	}
	client := mgr.GetClient()
	finder := mappings.NewResourceMappingsFinder(client)
	return &ReconcileVirtualMachineImport{client: client, scheme: mgr.GetScheme(), resourceMappingsFinder: finder, ocClient: tempClient}
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
	resourceMappingsFinder mappings.ResourceMappingsFinder
	ocClient               *templatev1.TemplateV1Client
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
	provider, err := r.initProvider(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer provider.Close()

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
	dvs := mapper.MapDisks()
	dvsDone := make(map[string]bool)
	for dvID := range dvs {
		foundDv := &cdiv1.DataVolume{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: dvID}, foundDv)
		if err != nil && errors.IsNotFound(err) {
			if err = r.createDataVolumes(provider, instance, dvs, vmName); err != nil {
				return reconcile.Result{}, err
			}
		} else if err == nil {
			// Set dataVolume as done, if it's in Succeeded state:
			if foundDv.Status.Phase == cdiv1.Succeeded {
				dvsDone[dvID] = true
				if err = r.manageDataVolumeState(instance, dvsDone, len(dvs)); err != nil {
					return reconcile.Result{}, err
				}

				// Cleanup if user don't want to start the VM
				if instance.Spec.StartVM == nil || !*instance.Spec.StartVM {
					if err := r.updateProgress(instance, progressDone); err != nil {
						return reconcile.Result{}, err
					}
					if err := r.afterSuccess(vmName, request.NamespacedName, provider); err != nil {
						return reconcile.Result{}, err
					}
				}
			}
		} else {
			return reconcile.Result{}, err
		}
	}

	if err = r.startVM(provider, instance, vmName); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVirtualMachineImport) createVM(provider provider.Provider, instance *v2vv1alpha1.VirtualMachineImport, mapper provider.Mapper) (string, error) {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)

	// Define VM spec
	template, err := provider.FindTemplate()
	var spec *kubevirtv1.VirtualMachine
	if err != nil {
		reqLogger.Info("No matching template was found for the virtual machine using empty vm definition")
		spec = mapper.CreateEmptyVM()
	} else {
		spec, err = provider.ProcessTemplate(template, *instance.Spec.TargetVMName)
		if err != nil {
			reqLogger.Info("Failed to process the template using empty vm definition")
			spec = mapper.CreateEmptyVM()
		}
	}
	vmSpec := mapper.MapVM(instance.Spec.TargetVMName, spec)

	// Update progress:
	if err = r.updateProgress(instance, progressStart); err != nil {
		return "", err
	}

	// Set VirtualMachineImport instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, vmSpec, r.scheme); err != nil {
		return "", err
	}

	// Update condition to creating VM:
	cond := conditions.NewProcessingCondition(string(v2vv1alpha1.CreatingTargetVM), "Creating virtual machine")
	if err = r.upsertStatusCondition(instanceNamespacedName, cond); err != nil {
		return "", err
	}

	// Create kubevirt VM from source VM:
	reqLogger.Info("Creating a new VM", "VM.Namespace", vmSpec.Namespace, "VM.Name", vmSpec.Name)
	if err = r.client.Create(context.TODO(), vmSpec); err != nil {
		// Update condition to failed state:
		cond = conditions.NewSucceededCondition(string(v2vv1alpha1.VMCreationFailed), fmt.Sprintf("Error while creating virtual machine: %s", err), corev1.ConditionFalse)
		err = r.upsertStatusCondition(instanceNamespacedName, cond)

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
				return err
			}
		} else if err == nil {
			if vmi.Status.Phase == kubevirtv1.Running || vmi.Status.Phase == kubevirtv1.Scheduled {
				instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
				cond := conditions.NewSucceededCondition(string(v2vv1alpha1.VirtualMachineRunning), "Virtual machine running", corev1.ConditionTrue)
				err = r.upsertStatusCondition(instanceNamespacedName, cond)
				if err != nil {
					return err
				}
				if err = r.updateProgress(instance, progressDone); err != nil {
					return err
				}
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
		cond := conditions.NewSucceededCondition(string(v2vv1alpha1.VirtualMachineReady), "Virtual machine disks import done", corev1.ConditionTrue)
		vmImportName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
		if err := r.upsertStatusCondition(vmImportName, cond); err != nil {
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

func (r *ReconcileVirtualMachineImport) createDataVolumes(provider provider.Provider, instance *v2vv1alpha1.VirtualMachineImport, dvs map[string]cdiv1.DataVolume, vmName types.NamespacedName) error {
	instanceNamespacedName := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	// Update condition to create VM:
	cond := conditions.NewProcessingCondition(string(v2vv1alpha1.CopyingDisks), "Copying virtual machine disks")
	err := r.upsertStatusCondition(instanceNamespacedName, cond)
	if err != nil {
		return err
	}
	// Update progress to copying disks:
	if err = r.updateProgress(instance, progressCopyingDisks); err != nil {
		return err
	}
	for _, dv := range dvs {
		if err := controllerutil.SetControllerReference(instance, &dv, r.scheme); err != nil {
			return err
		}

		err = r.client.Create(context.TODO(), &dv)
		if err != nil {
			// Update condition to failed:
			cond = conditions.NewSucceededCondition(string(v2vv1alpha1.DataVolumeCreationFailed), fmt.Sprintf("Data volume creation failed: %s", err), corev1.ConditionFalse)
			err = r.upsertStatusCondition(instanceNamespacedName, cond)
			if err != nil {
				return err
			}

			// Cleanup
			if err = r.afterFailure(instanceNamespacedName, provider); err != nil {
				return err
			}
			return err
		}
	}
	// Update datavolume in VM import CR status:
	if err = r.updateDVs(instanceNamespacedName, dvs); err != nil {
		return err
	}

	// Update VM spec with imported disks:
	vmDef := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, vmDef)
	if err != nil {
		return err
	}
	provider.UpdateVM(vmDef, dvs)
	err = r.client.Update(context.TODO(), vmDef)
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

func shouldValidate(vmiStatus *v2vv1alpha1.VirtualMachineImportStatus) bool {
	validatingCondition := conditions.FindConditionOfType(vmiStatus.Conditions, v2vv1alpha1.Validating)
	rulesCheckingCondition := conditions.FindConditionOfType(vmiStatus.Conditions, v2vv1alpha1.MappingRulesChecking)

	return isIncomplete(validatingCondition) || isIncomplete(rulesCheckingCondition)
}

func isIncomplete(condition *v2vv1alpha1.VirtualMachineImportCondition) bool {
	return condition == nil || condition.Status != corev1.ConditionTrue
}

func (r *ReconcileVirtualMachineImport) createProvider(vmi *v2vv1alpha1.VirtualMachineImport) (provider.Provider, error) {
	// The type of the provider is evaluated based on the source field from the CR
	if vmi.Spec.Source.Ovirt != nil {
		namespacedName := types.NamespacedName{Name: vmi.Name, Namespace: vmi.Namespace}
		provider := ovirtprovider.NewOvirtProvider(namespacedName, r.client, r.ocClient)
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

func (r *ReconcileVirtualMachineImport) upsertStatusConditions(vmiName types.NamespacedName, newConditions []v2vv1alpha1.VirtualMachineImportCondition) error {
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

	e := r.purgeOwnerReferences(vmName)
	if len(e) > 0 {
		errs = append(errs, e...)
	}

	if len(errs) > 0 {
		return foldErrors(errs, "Import success", vmiName)
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) purgeOwnerReferences(vmName types.NamespacedName) []error {
	var errs []error

	vm := kubevirtv1.VirtualMachine{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, &vm)
	if err != nil {
		errs = append(errs, err)
		// Stop here - we can't process further without a VM
		return errs
	}

	e := r.removeDataVolumesOwnerReferences(&vm)
	if len(e) > 0 {
		errs = append(errs, e...)
	}
	err = r.removeVMOwnerReference(&vm)
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

func (r *ReconcileVirtualMachineImport) removeDataVolumesOwnerReferences(vm *kubevirtv1.VirtualMachine) []error {
	var errs []error
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if v.DataVolume != nil {
			err := r.removeDataVolumeOwnerReference(vm.Namespace, v.DataVolume.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (r *ReconcileVirtualMachineImport) removeVMOwnerReference(vm *kubevirtv1.VirtualMachine) error {
	refs := vm.GetOwnerReferences()
	newRefs := removeControllerReference(refs)
	if len(newRefs) < len(refs) {
		vmCopy := vm.DeepCopy()
		vmCopy.SetOwnerReferences(newRefs)
		patch := client.MergeFrom(vm)
		return r.client.Patch(context.TODO(), vmCopy, patch)
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) removeDataVolumeOwnerReference(namespace string, dvName string) error {
	dv := &cdiv1.DataVolume{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: dvName}, dv)
	if err != nil {
		return err
	}

	refs := dv.GetOwnerReferences()
	newRefs := removeControllerReference(refs)
	if len(newRefs) < len(refs) {
		dvCopy := dv.DeepCopy()
		dvCopy.SetOwnerReferences(newRefs)
		patch := client.MergeFrom(dv)
		return r.client.Patch(context.TODO(), dvCopy, patch)
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

func removeControllerReference(refs []metav1.OwnerReference) []metav1.OwnerReference {
	for i := range refs {
		isController := refs[i].Controller
		if isController != nil && *isController {
			// There can be only one controller reference
			return append(refs[:i], refs[i+1:]...)
		}
	}
	return refs
}

func foldErrors(errs []error, prefix string, vmiName types.NamespacedName) error {
	message := ""
	for _, e := range errs {
		message = utils.WithMessage(message, e.Error())
	}
	return fmt.Errorf("%s clean-up for %v failed: %s", prefix, utils.ToLoggableResourceName(vmiName.Name, &vmiName.Namespace), message)
}

func (r *ReconcileVirtualMachineImport) upsertStatusCondition(vmiName types.NamespacedName, newCondition v2vv1alpha1.VirtualMachineImportCondition) error {
	return r.upsertStatusConditions(vmiName, []v2vv1alpha1.VirtualMachineImportCondition{newCondition})
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

func (r *ReconcileVirtualMachineImport) initProvider(instance *v2vv1alpha1.VirtualMachineImport) (provider.Provider, error) {
	// Fetch source provider secret
	sourceProviderSecretObj, err := r.fetchSecret(instance)
	if err != nil {
		return nil, err
	}

	provider, err := r.createProvider(instance)
	if err != nil {
		return nil, err
	}

	err = provider.Connect(sourceProviderSecretObj)
	if err != nil {
		return nil, err
	}

	// Load source VM:
	err = provider.LoadVM(instance.Spec.Source)
	if err != nil {
		return nil, err
	}

	// Load the external resource mapping
	resourceMapping, err := r.fetchResourceMapping(instance.Spec.ResourceMapping, instance.Namespace)
	if err != nil {
		//TODO: update Validating status condition
		return nil, err
	}

	// Prepare/merge the resourceMapping
	provider.PrepareResourceMapping(resourceMapping, instance.Spec.Source)

	return provider, nil
}

func (r *ReconcileVirtualMachineImport) validate(instance *v2vv1alpha1.VirtualMachineImport, provider provider.Provider) (bool, error) {
	logger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	if shouldValidate(&instance.Status) {
		conditions, err := provider.Validate()
		if err != nil {
			return true, err
		}
		err = r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, conditions)
		if err != nil {
			return true, err
		}
		if valid, message := shouldFailWith(conditions); !valid {
			logger.Info("Import blocked. " + message)
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
