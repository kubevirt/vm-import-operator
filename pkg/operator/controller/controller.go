package controller

import (
	"context"
	"fmt"

	sdkapi "github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/api"

	"github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"

	sdkr "github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/reconciler"
	"k8s.io/client-go/tools/record"

	"github.com/kelseyhightower/envconfig"
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/client-go/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	createVersionLabel          = "operator.v2v.kubevirt.io/createVersion"
	updateVersionLabel          = "operator.v2v.kubevirt.io/updateVersion"
	lastAppliedConfigAnnotation = "operator.v2v.kubevirt.io/lastAppliedConfiguration"
	MonitoringNamespace         = "MONITORING_NAMESPACE"

	// osConfigMapName represents the environment variable name that holds the OS config map name
	osConfigMapName = "OS_CONFIGMAP_NAME"

	// osConfigMapNamespace represents the environment variable name that holds the OS config map namespace
	osConfigMapNamespace = "OS_CONFIGMAP_NAMESPACE"
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
	Virtv2vImage           string `required:"true" split_words:"true"`
	DeployClusterResources string `required:"true" split_words:"true"`
	PullPolicy             string `required:"true" split_words:"true"`
	Namespace              string
	MonitoringNamespace    string
	InfraNodePlacement     *sdkapi.NodePlacement
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (*ReconcileVMImportConfig, error) {
	var operatorArgs OperatorArgs
	namespace, err := util.GetNamespace()
	if err != nil {
		return nil, err
	}

	err = envconfig.Process("", &operatorArgs)
	if err != nil {
		return nil, err
	}

	operatorArgs.Namespace = namespace

	log.Info("", "VARS", fmt.Sprintf("%+v", operatorArgs))

	scheme := mgr.GetScheme()
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: scheme,
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}

	cachingClient := mgr.GetClient()
	recorder := mgr.GetEventRecorderFor("virtualmachineimport-operator")
	r := &ReconcileVMImportConfig{
		client:         cachingClient,
		uncachedClient: uncachedClient,
		scheme:         scheme,
		namespace:      namespace,
		operatorArgs:   &operatorArgs,
		recorder:       recorder,
	}
	callbackDispatcher := callbacks.NewCallbackDispatcher(log, cachingClient, uncachedClient, scheme, namespace)
	r.reconciler = sdkr.NewReconciler(r, log, cachingClient, callbackDispatcher, scheme, createVersionLabel, updateVersionLabel, lastAppliedConfigAnnotation, 0, "vm-import-finalizer", recorder)

	r.registerHooks()
	addReconcileCallbacks(r)

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

	recorder   record.EventRecorder
	reconciler *sdkr.Reconciler
}

// Reconcile reads that state of the cluster for a VMImportConfig object and makes changes based on the state read
// and what is in the VMImportConfig.Spec
func (r *ReconcileVMImportConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling VMImportConfig")

	return r.reconciler.Reconcile(request, r.operatorArgs.OperatorVersion, reqLogger)
}

// SetController sets the controller dependency
func (r *ReconcileVMImportConfig) SetController(controller controller.Controller) {
	r.controller = controller
	r.reconciler.WithController(controller)
}

func (r *ReconcileVMImportConfig) add(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New("vm-import-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	r.SetController(c)

	if err = r.watchVMImportConfig(); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileVMImportConfig) watchVMImportConfig() error {
	return r.controller.Watch(&source.Kind{Type: &v2vv1.VMImportConfig{}}, &handler.EnqueueRequestForObject{})
}

func (r *ReconcileVMImportConfig) getOperatorArgs(cr *v2vv1.VMImportConfig) *OperatorArgs {
	result := *r.operatorArgs

	if cr != nil {
		if cr.Spec.ImagePullPolicy != "" {
			result.PullPolicy = string(cr.Spec.ImagePullPolicy)
		}
		result.InfraNodePlacement = &cr.Spec.Infra
	}

	operatorDeployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: result.Namespace, Name: "vm-import-operator"}
	if err := r.client.Get(context.TODO(), key, operatorDeployment); err == nil {
		operatorEnv := operatorDeployment.Spec.Template.Spec.Containers[0].Env
		for _, env := range operatorEnv {
			if env.Name == MonitoringNamespace {
				result.MonitoringNamespace = env.Value
			}
		}
	}

	return &result
}
