package virtualmachineimport

import (
	"context"
	langerr "errors"
	"fmt"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	ovirtprovider "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	keyAccess = "accessKeyId"
	keySecret = "secretKey"
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
	kubeClient, err := kubecli.GetKubevirtClientFromRESTConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "Unable to get KubeVirt client")
		panic("Controller cannot operate without KubeVirt")
	}
	client := mgr.GetClient()
	finder := mappings.NewResourceMappingsFinder(client)
	return &ReconcileVirtualMachineImport{client: client, scheme: mgr.GetScheme(), kubeClient: kubeClient, resourceMappingsFinder: finder}
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
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Ignore update func in case VM import succeded:
				vmimport := e.ObjectNew.(*v2vv1alpha1.VirtualMachineImport)
				for _, cond := range vmimport.Status.Conditions {
					if cond.Type == v2vv1alpha1.Succeeded {
						return false
					}
				}
				return true
			},
		},
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
		// We are interested only on Delete and Generic events.
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
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
		// We are interested only on Delete and Generic events.
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Ignore update func in case VM import succeded:
				dv := e.ObjectNew.(*cdiv1.DataVolume)
				if dv.Status.Phase == cdiv1.Succeeded {
					return false
				}
				return true
			},
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
	kubeClient             kubecli.KubevirtClient
	resourceMappingsFinder mappings.ResourceMappingsFinder
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

	// Fetch source provider secret
	sourceProviderSecretObj, err := r.fetchSecret(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	provider, err := r.createProvider(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = provider.Connect(sourceProviderSecretObj)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer provider.Close()

	// Load source VM:
	err = provider.LoadVM(instance.Spec.Source)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Load the external resource mapping
	resourceMapping, err := r.fetchResourceMapping(instance.Spec.ResourceMapping, instance.Namespace)
	if err != nil {
		//TODO: update Validating status condition
		return reconcile.Result{}, err
	}

	// Prepare/merge the resourceMapping
	provider.PrepareResourceMapping(resourceMapping, instance.Spec.Source)

	// Validate if it's needed at this stage of processing
	if shouldValidate(&instance.Status) {
		conditions, err := provider.Validate()
		if err != nil {
			return reconcile.Result{}, err
		}
		err = r.upsertStatusConditions(request.NamespacedName, conditions)
		if err != nil {
			return reconcile.Result{}, err
		}
		if valid, message := shouldFailWith(conditions); !valid {
			return reconcile.Result{}, langerr.New(message)
		}
	} else {
		reqLogger.Info("VirtualMachineImport has already been validated positively. Skipping re-validation")
	}

	// Stop VM
	err = provider.StopVM()
	if err != nil {
		return reconcile.Result{}, err
	}

	// Define VM spec
	mapper := provider.CreateMapper()
	vmSpec := mapper.MapVM(instance.Spec.TargetVMName)

	// Set VirtualMachineImport instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, vmSpec, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this VM already exists
	found := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: vmSpec.Name, Namespace: vmSpec.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create kubevirt VM from source VM:
		reqLogger.Info("Creating a new VM", "VM.Namespace", vmSpec.Namespace, "VM.Name", vmSpec.Name)
		err = r.client.Create(context.TODO(), vmSpec)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Secret with username/password for the image import:
		dvCreds := provider.GetDataVolumeCredentials()
		if err = r.ensureDVSecretExists(instance, vmSpec.Namespace, dvCreds); err != nil {
			return reconcile.Result{}, err
		}

		// CM containing CA for the image import:
		if err = r.ensureDVConfigMapExists(instance, vmSpec.Namespace, dvCreds); err != nil {
			return reconcile.Result{}, err
		}

		// Import disks:
		dvs := mapper.MapDisks()
		for _, dv := range dvs {
			_, err := r.kubeClient.CdiClient().CdiV1alpha1().DataVolumes(instance.Namespace).Create(&dv)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Update VM spec with disks:
		vmDef := &kubevirtv1.VirtualMachine{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmSpec.Namespace, Name: vmSpec.Name}, vmDef)
		if err != nil {
			return reconcile.Result{}, err
		}
		provider.UpdateVM(vmDef, dvs)
		err = r.client.Update(context.TODO(), vmDef)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// VM already exists - don't requeue
	reqLogger.Info("Skip reconcile: VM already exists", "VM.Namespace", found.Namespace, "VM.Name", found.Name)

	return reconcile.Result{}, nil
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

func (r *ReconcileVirtualMachineImport) ensureDVSecretExists(instance *v2vv1alpha1.VirtualMachineImport, namespace string, dvCreds provider.DataVolumeCredentials) error {
	secretObj := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: dvCreds.SecretName, Namespace: namespace}, secretObj)
	if err != nil && errors.IsNotFound(err) {
		dvSecret := createDVSecret(dvCreds, instance)
		return r.client.Create(context.TODO(), dvSecret)
	}
	return err
}

func (r *ReconcileVirtualMachineImport) ensureDVConfigMapExists(instance *v2vv1alpha1.VirtualMachineImport, namespace string, dvCreds provider.DataVolumeCredentials) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: dvCreds.ConfigMapName, Namespace: namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		dvCm := createDVConfigMap(dvCreds, instance)
		return r.client.Create(context.TODO(), dvCm)
	}
	return err
}

func createDVConfigMap(creds provider.DataVolumeCredentials, vmImport *v2vv1alpha1.VirtualMachineImport) *corev1.ConfigMap {
	// TODO: resource should be GC'ed after the import is done
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      creds.ConfigMapName,
			Namespace: vmImport.Namespace,
		},
		Data: map[string]string{
			"ca.pem": creds.CACertificate,
		},
	}
}

func createDVSecret(creds provider.DataVolumeCredentials, vmImport *v2vv1alpha1.VirtualMachineImport) *corev1.Secret {
	// TODO: resource should be GC'ed after the import is done
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      creds.SecretName,
			Namespace: vmImport.Namespace,
		},
		Data: map[string][]byte{
			keyAccess: []byte(creds.KeyAccess),
			keySecret: []byte(creds.KeySecret),
		},
	}
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
		provider := ovirtprovider.NewOvirtProvider(namespacedName, r.client, r.kubeClient)
		return &provider, nil
	}

	return nil, fmt.Errorf("Invalid source type. only Ovirt type is supported")
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
