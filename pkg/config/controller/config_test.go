package controller_test

import (
	"github.com/kubevirt/vm-import-operator/pkg/config"
	"github.com/kubevirt/vm-import-operator/pkg/config/controller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Controller config creator", func() {
	const (
		configMapName      = "map-name"
		configMapNamespace = "map-namespace"
	)
	configMap := corev1.ConfigMap{
		Data: map[string]string{
			"osConfigMap.name":      configMapName,
			"osConfigMap.namespace": configMapNamespace,
		},
	}
	cfg := controller.NewControllerConfigFrom(config.Config{ConfigMap: configMap})

	It("should create config with given config map", func() {
		Expect(cfg.ConfigMap).To(BeEquivalentTo(configMap))
	})

	It("should create config with os mapping map name", func() {
		Expect(cfg.OsConfigMapName()).To(BeEquivalentTo(configMapName))
	})

	It("should create config with os mapping map namespace", func() {
		Expect(cfg.OsConfigMapNamespace()).To(BeEquivalentTo(configMapNamespace))
	})
})
