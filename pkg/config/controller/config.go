package controller

import (
	"strconv"

	"github.com/kubevirt/vm-import-operator/pkg/config"
)

const (
	// OsConfigMapNamespaceKey defines the configuration key for the OS mapping config map namespace
	OsConfigMapNamespaceKey = "osConfigMap.namespace"

	// OsConfigMapNameKey defines the configuration key for the OS mapping config map name
	OsConfigMapNameKey = "osConfigMap.name"

	// WarmImportMaxFailuresKey defines the total number of failures to tolerate before failing the warm import
	WarmImportMaxFailuresKey     = "warmImport.maxFailures"
	warmImportMaxFailuresDefault = 10
	// WarmImportConsecutiveFailuresKey defines the number of consecutive failures to tolerate before failing the warm import
	WarmImportConsecutiveFailuresKey     = "warmImport.consecutiveFailures"
	warmImportConsecutiveFailuresDefault = 5
	// WarmImportIntervalMinutesKey defines how long to wait between warm import iterations
	WarmImportIntervalMinutesKey     = "warmImport.intervalMinutes"
	warmImportIntervalMinutesDefault = 60
	// ImportWithoutTemplateKey defines whether imports are permitted to run if an Openshift VM template can't be found.
	ImportWithoutTemplateKey         = "importWithoutTemplate"
	importWithoutTemplateDefault     = false
)

// ControllerConfig stores controller runtime configuration
type ControllerConfig struct {
	config.Config
}

// NewControllerConfigFrom creates new controller config from given Config
func NewControllerConfigFrom(config config.Config) ControllerConfig {
	return ControllerConfig{
		Config: config,
	}
}

// OsConfigMapNamespace provides namespace where the OS mapping ConfigMap resides. Empty string is returned when the namespace is not present.
func (c ControllerConfig) OsConfigMapNamespace() string {
	return c.ConfigMap.Data[OsConfigMapNamespaceKey]
}

// OsConfigMapName provides name of the the OS mapping ConfigMap. Empty string is returned when the name is not present.
func (c ControllerConfig) OsConfigMapName() string {
	return c.ConfigMap.Data[OsConfigMapNameKey]
}

func (c ControllerConfig) WarmImportMaxFailures() int {
	return c.getKeyAsInt(WarmImportMaxFailuresKey, warmImportMaxFailuresDefault, 0)
}

func (c ControllerConfig) WarmImportConsecutiveFailures() int {
	return c.getKeyAsInt(WarmImportConsecutiveFailuresKey, warmImportConsecutiveFailuresDefault, 0)
}

func (c ControllerConfig) WarmImportIntervalMinutes() int {
	return c.getKeyAsInt(WarmImportIntervalMinutesKey, warmImportIntervalMinutesDefault, 0)
}

func (c ControllerConfig) ImportWithoutTemplateEnabled() bool {
	return c.getKeyAsBool(ImportWithoutTemplateKey, importWithoutTemplateDefault)
}

func (c ControllerConfig) getKeyAsBool(key string, default_ bool) bool {
	raw := c.ConfigMap.Data[key]
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		parsed = default_
	}
	return parsed
}

func (c ControllerConfig) getKeyAsInt(key string, default_ int, floor int) int {
	raw := c.ConfigMap.Data[key]
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		parsed = default_
	}
	if parsed >= floor {
		return parsed
	} else {
		return floor
	}
}
