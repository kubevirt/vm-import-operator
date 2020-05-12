package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	jsondiff "github.com/appscode/jsonpatch"
	"github.com/blang/semver"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	"github.com/kelseyhightower/envconfig"
	vmimportv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	resources "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	createVersionLabel          = "operator.v2v.kubevirt.io/createVersion"
	updateVersionLabel          = "operator.v2v.kubevirt.io/updateVersion"
	lastAppliedConfigAnnotation = "operator.v2v.kubevirt.io/lastAppliedConfiguration"
)

var log = logf.Log.WithName("vmimport-operator")

// Add creates a new VMImport Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return r.add(mgr)
}

// OperatorArgs contains the required parameters to generate all namespaced resources
type OperatorArgs struct {
	OperatorVersion        string `required:"true" split_words:"true"`
	ControllerImage        string `required:"true" split_words:"true"`
	DeployClusterResources string `required:"true" split_words:"true"`
	PullPolicy             string `required:"true" split_words:"true"`
	Namespace              string
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (*ReconcileVMImportConfig, error) {
	var operatorArgs OperatorArgs
	namespace := util.GetNamespace()

	err := envconfig.Process("", &operatorArgs)
	if err != nil {
		return nil, err
	}

	operatorArgs.Namespace = namespace

	log.Info("", "VARS", fmt.Sprintf("%+v", operatorArgs))

	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}

	r := &ReconcileVMImportConfig{
		client:         mgr.GetClient(),
		uncachedClient: uncachedClient,
		scheme:         mgr.GetScheme(),
		namespace:      namespace,
		operatorArgs:   &operatorArgs,
	}

	return r, nil
}

var _ reconcile.Reconciler = &ReconcileVMImportConfig{}

// ReconcileVMImportConfig reconciles a VMImportConfig object
type ReconcileVMImportConfig struct {
	client client.Client

	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient client.Client
	scheme         *runtime.Scheme
	controller     controller.Controller

	namespace    string
	operatorArgs *OperatorArgs

	watching   bool
	watchMutex sync.Mutex
}

// Reconcile reads that state of the cluster for a VMImportConfig object and makes changes based on the state read
// and what is in the VMImportConfig.Spec
func (r *ReconcileVMImportConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling VMImportConfig")

	cr := &vmimportv1alpha1.VMImportConfig{}
	crKey := client.ObjectKey{Namespace: "", Name: request.NamespacedName.Name}
	if err := r.client.Get(context.TODO(), crKey, cr); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("VMImportConfig CR no longer exists")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// make sure we're watching eveything
	if err := r.watchDependantResources(cr); err != nil {
		return reconcile.Result{}, err
	}

	// mid delete
	if cr.DeletionTimestamp != nil {
		reqLogger.Info("Doing reconcile delete")
		return r.reconcileDelete(reqLogger, cr)
	}

	currentConditionValues := GetConditionValues(cr.Status.Conditions)
	reqLogger.Info("Doing reconcile update")

	res, err := r.reconcileUpdate(reqLogger, cr)
	if conditionsChanged(currentConditionValues, GetConditionValues(cr.Status.Conditions)) {
		if err := r.crUpdate(cr.Status.Phase, cr); err != nil {
			return reconcile.Result{}, err
		}
	}

	return res, err
}

// Compare condition maps and return true if any of the conditions changed, false otherwise.
func conditionsChanged(originalValues, newValues map[conditions.ConditionType]corev1.ConditionStatus) bool {
	if len(originalValues) != len(newValues) {
		return true
	}
	for k, v := range newValues {
		oldV, ok := originalValues[k]
		if !ok || oldV != v {
			return true
		}
	}
	return false
}

// GetConditionValues gets the conditions and put them into a map for easy comparison
func GetConditionValues(conditionList []conditions.Condition) map[conditions.ConditionType]corev1.ConditionStatus {
	result := make(map[conditions.ConditionType]corev1.ConditionStatus)
	for _, cond := range conditionList {
		result[cond.Type] = cond.Status
	}
	return result
}

func shouldTakeUpdatePath(logger logr.Logger, targetVersion, currentVersion string) (bool, error) {
	// if no current version, then this can't be an update
	if currentVersion == "" {
		return false, nil
	}

	if targetVersion == currentVersion {
		return false, nil
	}

	// semver doesn't like the 'v' prefix
	targetVersion = strings.TrimPrefix(targetVersion, "v")
	currentVersion = strings.TrimPrefix(currentVersion, "v")

	// our default position is that this is an update.
	// So if the target and current version do not
	// adhere to the semver spec, we assume by default the
	// update path is the correct path.
	shouldTakeUpdatePath := true
	target, err := semver.Make(targetVersion)
	if err == nil {
		current, err := semver.Make(currentVersion)
		if err == nil {
			if target.Compare(current) < 0 {
				err := fmt.Errorf("operator downgraded, will not reconcile")
				logger.Error(err, "", "current", current, "target", target)
				return false, err
			} else if target.Compare(current) == 0 {
				shouldTakeUpdatePath = false
			}
		}
	}

	return shouldTakeUpdatePath, nil
}

func (r *ReconcileVMImportConfig) checkUpgrade(logger logr.Logger, cr *vmimportv1alpha1.VMImportConfig) error {
	// should maybe put this in separate function
	if cr.Status.OperatorVersion != r.operatorArgs.OperatorVersion {
		cr.Status.OperatorVersion = r.operatorArgs.OperatorVersion
		cr.Status.TargetVersion = r.operatorArgs.OperatorVersion
		if err := r.crUpdate(cr.Status.Phase, cr); err != nil {
			return err
		}
	}

	isUpgrade, err := shouldTakeUpdatePath(logger, r.operatorArgs.OperatorVersion, cr.Status.ObservedVersion)
	if err != nil {
		return err
	}

	if isUpgrade && cr.Status.Phase != vmimportv1alpha1.PhaseUpgrading {
		logger.Info("Observed version is not target version. Begin upgrade", "Observed version ", cr.Status.ObservedVersion, "TargetVersion", r.operatorArgs.OperatorVersion)
		MarkCrUpgradeHealingDegraded(cr, "UpgradeStarted", fmt.Sprintf("Started upgrade to version %s", r.operatorArgs.OperatorVersion))
		if err := r.crUpdate(vmimportv1alpha1.PhaseUpgrading, cr); err != nil {
			return err
		}
	}

	return nil
}

// MarkCrUpgradeHealingDegraded marks the passed CR as upgrading and degraded.
func MarkCrUpgradeHealingDegraded(cr *vmimportv1alpha1.VMImportConfig, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionAvailable,
		Status: corev1.ConditionTrue,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionProgressing,
		Status: corev1.ConditionTrue,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionDegraded,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
}

func newDefaultInstance(obj runtime.Object) runtime.Object {
	typ := reflect.ValueOf(obj).Elem().Type()
	return reflect.New(typ).Interface().(runtime.Object)
}

func (r *ReconcileVMImportConfig) reconcileUpdate(logger logr.Logger, cr *vmimportv1alpha1.VMImportConfig) (reconcile.Result, error) {
	if err := r.checkUpgrade(logger, cr); err != nil {
		return reconcile.Result{}, err
	}

	resources, err := r.getAllResources(cr)
	if err != nil {
		return reconcile.Result{}, err
	}

	var allErrors []error
	for _, desiredRuntimeObj := range resources {
		desiredMetaObj := desiredRuntimeObj.(metav1.Object)
		currentRuntimeObj := newDefaultInstance(desiredRuntimeObj)

		key := client.ObjectKey{
			Namespace: desiredMetaObj.GetNamespace(),
			Name:      desiredMetaObj.GetName(),
		}
		err = r.client.Get(context.TODO(), key, currentRuntimeObj)

		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			setLastAppliedConfiguration(desiredMetaObj)
			setLabel(createVersionLabel, r.operatorArgs.OperatorVersion, desiredMetaObj)

			if err = controllerutil.SetControllerReference(cr, desiredMetaObj, r.scheme); err != nil {
				return reconcile.Result{}, err
			}

			currentRuntimeObj = desiredRuntimeObj.DeepCopyObject()
			if err = r.client.Create(context.TODO(), currentRuntimeObj); err != nil {
				logger.Error(err, "")
				allErrors = append(allErrors, err)
				continue
			}

			logger.Info("Resource created",
				"namespace", desiredMetaObj.GetNamespace(),
				"name", desiredMetaObj.GetName(),
				"type", fmt.Sprintf("%T", desiredMetaObj))
		} else {
			currentRuntimeObjCopy := currentRuntimeObj.DeepCopyObject()
			currentMetaObj := currentRuntimeObj.(metav1.Object)

			if !r.isMutable(currentRuntimeObj) {
				setLastAppliedConfiguration(desiredMetaObj)

				// overwrite currentRuntimeObj
				currentRuntimeObj, err = mergeObject(desiredRuntimeObj, currentRuntimeObj)
				if err != nil {
					return reconcile.Result{}, err
				}
				currentMetaObj = currentRuntimeObj.(metav1.Object)
			}

			if !reflect.DeepEqual(currentRuntimeObjCopy, currentRuntimeObj) {
				logJSONDiff(logger, currentRuntimeObjCopy, currentRuntimeObj)

				setLabel(updateVersionLabel, r.operatorArgs.OperatorVersion, currentMetaObj)

				if err = r.client.Update(context.TODO(), currentRuntimeObj); err != nil {
					logger.Error(err, "")
					allErrors = append(allErrors, err)
					continue
				}

				logger.Info("Resource updated",
					"namespace", desiredMetaObj.GetNamespace(),
					"name", desiredMetaObj.GetName(),
					"type", fmt.Sprintf("%T", desiredMetaObj))
			} else {
				logger.V(3).Info("Resource unchanged",
					"namespace", desiredMetaObj.GetNamespace(),
					"name", desiredMetaObj.GetName(),
					"type", fmt.Sprintf("%T", desiredMetaObj))
			}
		}
	}

	if len(allErrors) > 0 {
		return reconcile.Result{}, fmt.Errorf("reconcile encountered %d errors", len(allErrors))
	}

	degraded, err := r.checkDegraded(logger, cr)
	if err != nil {
		return reconcile.Result{}, err
	}

	if cr.Status.Phase != vmimportv1alpha1.PhaseDeployed && !r.isUpgrading(cr) && !degraded {
		//We are not moving to Deployed phase until new operator deployment is ready in case of Upgrade
		cr.Status.ObservedVersion = r.operatorArgs.OperatorVersion
		MarkCrHealthyMessage(cr, "DeployCompleted", "Deployment Completed")
		if err = r.crUpdate(vmimportv1alpha1.PhaseDeployed, cr); err != nil {
			return reconcile.Result{}, err
		}

		logger.Info("Successfully entered Deployed state")
	}

	if !degraded && r.isUpgrading(cr) {
		logger.Info("Completing upgrade process...")

		if err = r.completeUpgrade(logger, cr); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func mergeObject(desiredObj, currentObj runtime.Object) (runtime.Object, error) {
	desiredObj = desiredObj.DeepCopyObject()
	desiredMetaObj := desiredObj.(metav1.Object)
	currentMetaObj := currentObj.(metav1.Object)

	v, ok := currentMetaObj.GetAnnotations()[lastAppliedConfigAnnotation]
	if !ok {
		log.Info("Resource missing last applied config", "resource", currentMetaObj)
	}

	original := []byte(v)

	// setting the timestamp saves unnecessary updates because creation timestamp is nulled
	desiredMetaObj.SetCreationTimestamp(currentMetaObj.GetCreationTimestamp())
	modified, err := json.Marshal(desiredObj)
	if err != nil {
		return nil, err
	}

	current, err := json.Marshal(currentObj)
	if err != nil {
		return nil, err
	}

	preconditions := []mergepatch.PreconditionFunc{
		mergepatch.RequireKeyUnchanged("apiVersion"),
		mergepatch.RequireKeyUnchanged("kind"),
		mergepatch.RequireMetadataKeyUnchanged("name"),
	}

	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current, preconditions...)
	if err != nil {
		return nil, err
	}

	newCurrent, err := jsonpatch.MergePatch(current, patch)
	if err != nil {
		return nil, err
	}

	result := newDefaultInstance(currentObj)
	if err = json.Unmarshal(newCurrent, result); err != nil {
		return nil, err
	}

	return result, nil
}

func logJSONDiff(logger logr.Logger, objA, objB interface{}) {
	aBytes, _ := json.Marshal(objA)
	bBytes, _ := json.Marshal(objB)
	patches, _ := jsondiff.CreatePatch(aBytes, bBytes)
	pBytes, _ := json.Marshal(patches)
	logger.Info("DIFF", "obj", objA, "patch", string(pBytes))
}

func (r *ReconcileVMImportConfig) isUpgrading(cr *vmimportv1alpha1.VMImportConfig) bool {
	return cr.Status.ObservedVersion != "" && cr.Status.ObservedVersion != cr.Status.TargetVersion
}

func (r *ReconcileVMImportConfig) completeUpgrade(logger logr.Logger, cr *vmimportv1alpha1.VMImportConfig) error {
	if err := r.cleanupUnusedResources(logger, cr); err != nil {
		return err
	}

	previousVersion := cr.Status.ObservedVersion
	cr.Status.ObservedVersion = r.operatorArgs.OperatorVersion

	MarkCrHealthyMessage(cr, "DeployCompleted", "Deployment Completed")
	if err := r.crUpdate(vmimportv1alpha1.PhaseDeployed, cr); err != nil {
		return err
	}

	logger.Info("Successfully finished Upgrade and entered Deployed state", "from version", previousVersion, "to version", cr.Status.ObservedVersion)

	return nil
}

// MarkCrHealthyMessage marks the passed in CR as healthy.
func MarkCrHealthyMessage(cr *vmimportv1alpha1.VMImportConfig, reason, message string) {
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:    conditions.ConditionAvailable,
		Status:  corev1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionProgressing,
		Status: corev1.ConditionFalse,
	})
	conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
		Type:   conditions.ConditionDegraded,
		Status: corev1.ConditionFalse,
	})
}

func (r *ReconcileVMImportConfig) cleanupUnusedResources(logger logr.Logger, cr *vmimportv1alpha1.VMImportConfig) error {
	desiredResources, err := r.getAllResources(cr)
	if err != nil {
		return err
	}

	listTypes := []runtime.Object{
		&extv1beta1.CustomResourceDefinitionList{},
		&rbacv1.ClusterRoleBindingList{},
		&rbacv1.ClusterRoleList{},
		&appsv1.DeploymentList{},
		&rbacv1.RoleBindingList{},
		&rbacv1.RoleList{},
		&corev1.ServiceAccountList{},
	}

	ls, err := labels.Parse(createVersionLabel)
	if err != nil {
		return err
	}

	for _, lt := range listTypes {
		lo := &client.ListOptions{LabelSelector: ls}

		if err := r.client.List(context.TODO(), lt, lo); err != nil {
			logger.Error(err, "Error listing resources")
			return err
		}

		sv := reflect.ValueOf(lt).Elem()
		iv := sv.FieldByName("Items")

		for i := 0; i < iv.Len(); i++ {
			found := false
			observedObj := iv.Index(i).Addr().Interface().(runtime.Object)
			observedMetaObj := observedObj.(metav1.Object)

			for _, desiredObj := range desiredResources {
				if sameResource(observedObj, desiredObj) {
					found = true
					break
				}
			}

			if !found && metav1.IsControlledBy(observedMetaObj, cr) {
				logger.Info("Deleting  ", "type", reflect.TypeOf(observedObj), "Name", observedMetaObj.GetName())
				err = r.client.Delete(context.TODO(), observedObj, &client.DeleteOptions{
					PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
				})
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	return nil
}

func (r *ReconcileVMImportConfig) reconcileDelete(logger logr.Logger, cr *vmimportv1alpha1.VMImportConfig) (reconcile.Result, error) {
	if cr.Status.Phase != vmimportv1alpha1.PhaseDeleting {
		if err := r.crUpdate(vmimportv1alpha1.PhaseDeleting, cr); err != nil {
			return reconcile.Result{}, err
		}
	}

	deployments, err := r.getAllDeployments(cr)
	if err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("Deleting VMImport deployment")

	for _, deployment := range deployments {
		if !isControllerDeployment(deployment) {
			continue
		}
		err := r.client.Delete(context.TODO(), deployment, &client.DeleteOptions{
			PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
		})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Error deleting vm import controller deployment")
			return reconcile.Result{}, err
		}
	}

	err = deleteRelatedResources(logger, r.client)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.crUpdate(vmimportv1alpha1.PhaseDeleted, cr); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func deleteRelatedResources(logger logr.Logger, c client.Client) error {
	object := &corev1.PodList{}
	ls, err := labels.Parse("v2v.kubevirt.io")
	if err != nil {
		return err
	}

	options := &client.ListOptions{
		LabelSelector: ls,
	}

	if err := c.List(context.TODO(), object, options); err != nil {
		logger.Error(err, "Error listing resources")
		return err
	}

	sv := reflect.ValueOf(object).Elem()
	iv := sv.FieldByName("Items")

	for i := 0; i < iv.Len(); i++ {
		obj := iv.Index(i).Addr().Interface().(runtime.Object)
		logger.Info("Deleting", "type", reflect.TypeOf(obj), "obj", obj)
		if err := c.Delete(context.TODO(), obj); err != nil {
			logger.Error(err, "Error deleting a resource")
			return err
		}
	}

	return nil
}

func isControllerDeployment(d *appsv1.Deployment) bool {
	return d.Name == "vm-import-deployment"
}

func (r *ReconcileVMImportConfig) crUpdate(phase vmimportv1alpha1.VMImportPhase, cr *vmimportv1alpha1.VMImportConfig) error {
	cr.Status.Phase = phase
	return r.client.Update(context.TODO(), cr)
}

func (r *ReconcileVMImportConfig) checkDegraded(logger logr.Logger, cr *vmimportv1alpha1.VMImportConfig) (bool, error) {
	degraded := false

	deployments, err := r.getAllDeployments(cr)
	if err != nil {
		return true, err
	}

	for _, deployment := range deployments {
		key := client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name}

		if err = r.client.Get(context.TODO(), key, deployment); err != nil {
			return true, err
		}

		if !checkDeploymentReady(deployment) {
			degraded = true
			break
		}
	}

	logger.Info("VMImport degraded check", "Degraded", degraded)

	// If deployed and degraded, mark degraded, otherwise we are still deploying or not degraded.
	if degraded && cr.Status.Phase == vmimportv1alpha1.PhaseDeployed {
		conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
			Type:   conditions.ConditionDegraded,
			Status: corev1.ConditionTrue,
		})
	} else {
		conditions.SetStatusCondition(&cr.Status.Conditions, conditions.Condition{
			Type:   conditions.ConditionDegraded,
			Status: corev1.ConditionFalse,
		})
	}

	logger.Info("Finished degraded check", "conditions", cr.Status.Conditions)
	return degraded, nil
}

func checkDeploymentReady(deployment *appsv1.Deployment) bool {
	desiredReplicas := deployment.Spec.Replicas
	if desiredReplicas == nil {
		desiredReplicas = &[]int32{1}[0]
	}

	if *desiredReplicas != deployment.Status.Replicas ||
		deployment.Status.Replicas != deployment.Status.ReadyReplicas {
		return false
	}

	return true
}

func (r *ReconcileVMImportConfig) add(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New("vm-import-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	r.controller = c

	if err = r.watchVMImportConfig(); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileVMImportConfig) watchVMImportConfig() error {
	return r.controller.Watch(&source.Kind{Type: &vmimportv1alpha1.VMImportConfig{}}, &handler.EnqueueRequestForObject{})
}

func (r *ReconcileVMImportConfig) watchDependantResources(cr *vmimportv1alpha1.VMImportConfig) error {
	r.watchMutex.Lock()
	defer r.watchMutex.Unlock()

	if r.watching {
		return nil
	}

	resources, err := r.getAllResources(cr)
	if err != nil {
		return err
	}

	if err = r.watchResourceTypes(resources); err != nil {
		return err
	}

	r.watching = true

	return nil
}

func (r *ReconcileVMImportConfig) getAllDeployments(cr *vmimportv1alpha1.VMImportConfig) ([]*appsv1.Deployment, error) {
	var result []*appsv1.Deployment

	resources, err := r.getAllResources(cr)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		if deployment, ok := resource.(*appsv1.Deployment); ok {
			result = append(result, deployment)
		}
	}

	return result, nil
}

func (r *ReconcileVMImportConfig) getOperatorArgs(cr *vmimportv1alpha1.VMImportConfig) *OperatorArgs {
	result := *r.operatorArgs

	if cr != nil {
		if cr.Spec.ImagePullPolicy != "" {
			result.PullPolicy = string(cr.Spec.ImagePullPolicy)
		}
	}

	return &result
}

func (r *ReconcileVMImportConfig) getAllResources(cr *vmimportv1alpha1.VMImportConfig) ([]runtime.Object, error) {
	var resources []runtime.Object

	if deployClusterResources() {
		rs := createCRDResources()
		resources = append(resources, rs...)
	}

	nsrs := createControllerResources(r.getOperatorArgs(cr))
	resources = append(resources, nsrs...)

	return resources, nil
}

func createControllerResources(args *OperatorArgs) []runtime.Object {
	return []runtime.Object{
		resources.CreateServiceAccount(),
		resources.CreateRoleBinding(args.Namespace),
		resources.CreateRole(),
		resources.CreateControllerDeployment("vm-import-deployment", args.Namespace, args.ControllerImage, args.PullPolicy, int32(1)),
	}
}

func createCRDResources() []runtime.Object {
	return []runtime.Object{
		resources.CreateResourceMapping(),
		resources.CreateVMImport(),
	}
}

func deployClusterResources() bool {
	return strings.ToLower(os.Getenv("DEPLOY_CLUSTER_RESOURCES")) != "false"
}

func (r *ReconcileVMImportConfig) watchResourceTypes(resources []runtime.Object) error {
	types := map[reflect.Type]bool{}

	for _, resource := range resources {
		t := reflect.TypeOf(resource)
		if types[t] {
			continue
		}

		eventHandler := &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &vmimportv1alpha1.VMImportConfig{},
		}

		if err := r.controller.Watch(&source.Kind{Type: resource}, eventHandler); err != nil {
			if meta.IsNoMatchError(err) {
				log.Info("No match for type, NOT WATCHING", "type", t)
				continue
			}
			return err
		}

		log.Info("Watching", "type", t)

		types[t] = true
	}

	return nil
}

func setLabel(key, value string, obj metav1.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string))
	}
	obj.GetLabels()[key] = value
}

func setLastAppliedConfiguration(obj metav1.Object) error {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}

	obj.GetAnnotations()[lastAppliedConfigAnnotation] = string(bytes)

	return nil
}

func sameResource(obj1, obj2 runtime.Object) bool {
	metaObj1 := obj1.(metav1.Object)
	metaObj2 := obj2.(metav1.Object)

	if reflect.TypeOf(obj1) != reflect.TypeOf(obj2) ||
		metaObj1.GetNamespace() != metaObj2.GetNamespace() ||
		metaObj1.GetName() != metaObj2.GetName() {
		return false
	}

	return true
}

// this is used for testing.  wish this a helper function in test file instead of member
func (r *ReconcileVMImportConfig) crSetVersion(cr *vmimportv1alpha1.VMImportConfig, version string) error {
	phase := vmimportv1alpha1.PhaseDeployed
	if version == "" {
		phase = vmimportv1alpha1.VMImportPhase("")
	}
	cr.Status.ObservedVersion = version
	cr.Status.OperatorVersion = version
	cr.Status.TargetVersion = version
	return r.crUpdate(phase, cr)
}

func (r *ReconcileVMImportConfig) isMutable(obj runtime.Object) bool {
	switch obj.(type) {
	case *corev1.ConfigMap, *corev1.Secret, *rbacv1.RoleBinding, *rbacv1.Role:
		return true
	}
	return false
}
