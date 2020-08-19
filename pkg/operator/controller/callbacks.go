package controller

import (
	"context"
	"reflect"

	"github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"

	resources "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func addReconcileCallbacks(r *ReconcileVMImportConfig) {
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteControllerDeployment)
}

func reconcileDeleteControllerDeployment(args *callbacks.ReconcileCallbackArgs) error {
	switch args.State {
	case callbacks.ReconcileStatePostDelete, callbacks.ReconcileStateOperatorDelete:
	default:
		return nil
	}

	var deployment *appsv1.Deployment
	if args.DesiredObject != nil {
		deployment = args.DesiredObject.(*appsv1.Deployment)
	} else if args.CurrentObject != nil {
		deployment = args.CurrentObject.(*appsv1.Deployment)
	} else {
		args.Logger.Info("Received callback with no desired/current object")
		return nil
	}

	if !isControllerDeployment(deployment) {
		return nil
	}

	args.Logger.Info("Deleting vm-import-controller deployment and all related pods")

	object := &corev1.PodList{}
	ls, err := labels.Parse("v2v.kubevirt.io")
	if err != nil {
		return err
	}

	options := &client.ListOptions{
		LabelSelector: ls,
	}

	if err := args.Client.List(context.TODO(), object, options); err != nil {
		args.Logger.Error(err, "Error listing resources")
		return err
	}

	sv := reflect.ValueOf(object).Elem()
	iv := sv.FieldByName("Items")

	for i := 0; i < iv.Len(); i++ {
		obj := iv.Index(i).Addr().Interface().(runtime.Object)
		args.Logger.Info("Deleting", "type", reflect.TypeOf(obj), "obj", obj)
		if err := args.Client.Delete(context.TODO(), obj); err != nil {
			args.Logger.Error(err, "Error deleting a resource")
			return err
		}
	}

	return nil
}

func isControllerDeployment(d *appsv1.Deployment) bool {
	return d.Name == resources.ControllerName
}
