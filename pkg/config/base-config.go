package config

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Config stores runtime configuration
type Config struct {
	ConfigMap v1.ConfigMap
}

// New creates new Config
func New(configMap v1.ConfigMap) Config {
	return Config{configMap}
}

// String returns string representation of the config
func (c *Config) String() string {
	return c.ConfigMap.String()
}

// Provider is responsible for providing the current config
type Provider struct {
	configMapInformer cache.SharedIndexInformer
	namespace         string
}

// NewConfigProvider creates config provider that will ensure that the provided config is up to date
func NewConfigProvider(stopCh chan struct{}, clientset kubernetes.Interface, configMapNamespace string, configMapName string) Provider {
	tl := func(opts *metav1.ListOptions) {
		opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", configMapName).String()
	}
	configMapInformer := v1informers.NewFilteredConfigMapInformer(clientset, configMapNamespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, tl)
	go configMapInformer.Run(stopCh)

	cache.WaitForCacheSync(stopCh, configMapInformer.HasSynced)

	return Provider{namespace: configMapNamespace, configMapInformer: configMapInformer}
}

// GetBareConfig provides the most current config
func (cp *Provider) GetBareConfig(configMapName string) (Config, error) {
	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
	}
	item, exists, err := cp.configMapInformer.GetStore().GetByKey(cp.namespace + "/" + configMapName)
	if err != nil {
		return Config{}, err
	}
	if exists {
		configMap = *item.(*v1.ConfigMap)
	}
	return Config{ConfigMap: configMap}, nil
}
