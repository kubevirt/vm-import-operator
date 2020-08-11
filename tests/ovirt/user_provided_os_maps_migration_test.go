package ovirt

import (
	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("OS Mapping ConfigMap name and namespace", func() {

	var f = fwk.NewFrameworkOrDie("os-maps-migration")

	It("should be copied from operator ENV to config map", func() {
		controllerConfigMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get(ctrlConfig.ConfigMapName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(controllerConfigMap.Data[ctrlConfig.OsConfigMapNameKey]).To(BeEquivalentTo("vmimport-os-mapper"))
		Expect(controllerConfigMap.Data[ctrlConfig.OsConfigMapNamespaceKey]).To(BeEquivalentTo("os-mapping"))
	})
})
