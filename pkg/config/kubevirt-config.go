package config

import (
	"strings"

	v1 "k8s.io/api/core/v1"
)

const (
	featureGatesKey = "feature-gates"

	liveMigrationGate = "LiveMigration"
)

// NewKubeVirtConfig creates new KubeVirt and initializes it with given configMap
func NewKubeVirtConfig(configMap v1.ConfigMap) KubeVirtConfig {
	config := KubeVirtConfig{
		configMap: configMap,
	}
	if featureGates := strings.TrimSpace(configMap.Data[featureGatesKey]); featureGates != "" {
		config.FeatureGates = featureGates
	}
	return config
}

// KubeVirtConfig stores KubeVirt runtime configuration
type KubeVirtConfig struct {
	FeatureGates string
	configMap    v1.ConfigMap
}

// ConfigMap returns plain KubeVirt config map
func (c *KubeVirtConfig) ConfigMap() v1.ConfigMap {
	return c.configMap
}

// LiveMigrationEnabled returns true if LiveMigration KubeVirt feature gate is enabled
func (c *KubeVirtConfig) LiveMigrationEnabled() bool {
	return c.isFeatureGateEnabled(liveMigrationGate)
}

// String returns string representation of the config
func (c *KubeVirtConfig) String() string {
	return c.configMap.String()
}

func (c *KubeVirtConfig) isFeatureGateEnabled(featureGate string) bool {
	return strings.Contains(c.FeatureGates, featureGate)
}
