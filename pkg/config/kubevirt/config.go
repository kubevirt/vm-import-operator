package kubevirt

import (
	"strings"

	"github.com/kubevirt/vm-import-operator/pkg/config"

	v1 "k8s.io/api/core/v1"
)

const (
	featureGatesKey = "feature-gates"

	liveMigrationGate = "LiveMigration"

	importWithoutTemplateGate = "ImportWithoutTemplate"
)

// NewKubeVirtConfig creates new KubeVirt and initializes it with given configMap
func NewKubeVirtConfig(configMap v1.ConfigMap) KubeVirtConfig {
	return NewKubeVirtConfigFrom(config.Config{ConfigMap: configMap})
}

// NewKubeVirtConfigFrom creates new KubeVirt and initializes it with given Config
func NewKubeVirtConfigFrom(config config.Config) KubeVirtConfig {
	kubeVirtConfig := KubeVirtConfig{
		Config: config,
	}
	if featureGates := strings.TrimSpace(config.ConfigMap.Data[featureGatesKey]); featureGates != "" {
		kubeVirtConfig.FeatureGates = featureGates
	}
	return kubeVirtConfig
}

// KubeVirtConfig stores KubeVirt runtime configuration
type KubeVirtConfig struct {
	config.Config
	FeatureGates string
}

// ImportWithoutTemplateEnabled returns true if ImportWithoutTemplate KubeVirt feature gate is enabled
func (c *KubeVirtConfig) ImportWithoutTemplateEnabled() bool {
	return c.isFeatureGateEnabled(importWithoutTemplateGate)
}

func (c *KubeVirtConfig) isFeatureGateEnabled(featureGate string) bool {
	return strings.Contains(c.FeatureGates, featureGate)
}
