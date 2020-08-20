package controller

import (
	"context"
	"fmt"
	"os"
	"strings"

	resources "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"

	sdkapi "github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/api"
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Create creates empty CR
func (r *ReconcileVMImportConfig) Create() controllerutil.Object {
	return &v2vv1.VMImportConfig{}
}

// Status extracts status from the cr
func (r *ReconcileVMImportConfig) Status(object runtime.Object) *sdkapi.Status {
	return &object.(*v2vv1.VMImportConfig).Status.Status
}

// GetAllResources provides all resources managed by the cr
func (r *ReconcileVMImportConfig) GetAllResources(cr runtime.Object) ([]runtime.Object, error) {
	return r.getAllResources(cr.(*v2vv1.VMImportConfig))
}

// GetDependantResourcesListObjects returns resource list objects of dependant resources
func (r *ReconcileVMImportConfig) GetDependantResourcesListObjects() []runtime.Object {
	return []runtime.Object{
		&extv1.CustomResourceDefinitionList{},
		&rbacv1.ClusterRoleBindingList{},
		&rbacv1.ClusterRoleList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceAccountList{},
	}
}

// IsCreating checks whether creation of the managed resources will be executed
func (r *ReconcileVMImportConfig) IsCreating(cr controllerutil.Object) (bool, error) {
	vmiconfig := cr.(*v2vv1.VMImportConfig)
	return vmiconfig.Status.Conditions == nil || len(vmiconfig.Status.Conditions) == 0, nil
}

func (r *ReconcileVMImportConfig) getAllResources(cr *v2vv1.VMImportConfig) ([]runtime.Object, error) {
	var resultingResources []runtime.Object

	if deployClusterResources() {
		rs := createCRDResources()
		resultingResources = append(resultingResources, rs...)
	}

	nsrs := createControllerResources(r.getOperatorArgs(cr))
	resultingResources = append(resultingResources, nsrs...)

	return resultingResources, nil
}

func createControllerResources(args *OperatorArgs) []runtime.Object {
	objs := []runtime.Object{
		resources.CreateServiceAccount(args.Namespace),
		resources.CreateControllerRole(),
		resources.CreateControllerRoleBinding(args.Namespace),
		resources.CreateControllerDeployment(resources.ControllerName, args.Namespace, args.ControllerImage, args.Virtv2vImage, args.PullPolicy, int32(1), args.InfraNodePlacement),
	}
	// Add metrics objects if servicemonitor is available:
	if ok, err := hasServiceMonitor(); ok && err == nil {
		objs = append(objs,
			resources.CreateMetricsService(args.Namespace),
			resources.CreateServiceMonitor(args.MonitoringNamespace, args.Namespace),
		)
	}

	return objs
}

// hasServiceMonitor checks if ServiceMonitor is registered in the cluster.
func hasServiceMonitor() (bool, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return false, fmt.Errorf("Can't load restconfig")
	}

	dc := discovery.NewDiscoveryClientForConfigOrDie(cfg)
	apiVersion := "monitoring.coreos.com/v1"
	kind := "ServiceMonitor"

	return k8sutil.ResourceExists(dc, apiVersion, kind)
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

func (r *ReconcileVMImportConfig) getOsMappingConfigMapName(namespace string) *types.NamespacedName {
	var configMapName, configMapNamespace string
	operatorDeployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: namespace, Name: "vm-import-operator"}
	if err := r.client.Get(context.TODO(), key, operatorDeployment); err == nil {
		operatorEnv := r.findVMImportOperatorContainer(*operatorDeployment).Env
		for _, env := range operatorEnv {
			if env.Name == osConfigMapName {
				configMapName = env.Value
			}
			if env.Name == osConfigMapNamespace {
				configMapNamespace = env.Value
			}
		}
	}
	if configMapName == "" && configMapNamespace == "" {
		return nil
	}
	return &types.NamespacedName{Name: configMapName, Namespace: configMapNamespace}
}

func (r *ReconcileVMImportConfig) findVMImportOperatorContainer(operatorDeployment appsv1.Deployment) corev1.Container {
	for _, container := range operatorDeployment.Spec.Template.Spec.Containers {
		if container.Name == "vm-import-operator" {
			return container
		}
	}

	log.Info("vm-import-operator container not found", "deployment", operatorDeployment.Name)
	return corev1.Container{}
}
