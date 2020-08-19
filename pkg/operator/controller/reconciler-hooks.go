package controller

import (
	"context"

	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileVMImportConfig) updateControllerConfig(cr controllerutil.Object) error {
	configMap := corev1.ConfigMap{}
	configMapID := client.ObjectKey{Namespace: r.namespace, Name: ctrlConfig.ConfigMapName}
	if err := r.client.Get(context.TODO(), configMapID, &configMap); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		migratingFromEnv := false
		if namespacedName := r.getOsMappingConfigMapName(r.namespace); namespacedName != nil {
			configMap.Data = make(map[string]string)
			configMap.Data[ctrlConfig.OsConfigMapNameKey] = namespacedName.Name
			configMap.Data[ctrlConfig.OsConfigMapNamespaceKey] = namespacedName.Namespace
			migratingFromEnv = true
		}
		configMap.Name = configMapID.Name
		configMap.Namespace = configMapID.Namespace
		if err = r.client.Create(context.TODO(), &configMap); err != nil {
			return err
		}
		if migratingFromEnv {
			r.recorder.Eventf(cr, corev1.EventTypeWarning, "OSConfigMapMigrated", "OS mapping config map configuration has been migrated from environment variables to the controller config map. %s and %s env variables can be removed from the vm-import-operator deployment", osConfigMapName, osConfigMapNamespace)
		}
	}
	return nil
}

func (r *ReconcileVMImportConfig) registerHooks() {
	r.reconciler.
		WithControllerConfigUpdater(r.updateControllerConfig)
}
