package config

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	configMapName = "kubevirt-config"
)

// KubeVirtConfigProvider defines KubeVirt config access operations
type KubeVirtConfigProvider interface {
	GetConfig() (KubeVirtConfig, error)
}

// KubeVirtClusterConfigProvider is responsible for providing the current KubeVirt cluster config
type KubeVirtClusterConfigProvider struct {
	configMapInformer cache.SharedIndexInformer
	namespace         string
}

// NewKubeVirtConfigProvider creates new KubeVirt config provider that will ensure that the provided config is up to date
func NewKubeVirtConfigProvider(stopCh chan struct{}, clientset kubernetes.Interface, kubevirtNamespace string) KubeVirtClusterConfigProvider {
	tl := func(opts *metav1.ListOptions) {
		opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", "kubevirt-config").String()
	}
	configMapInformer := v1informers.NewFilteredConfigMapInformer(clientset, kubevirtNamespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, tl)
	go configMapInformer.Run(stopCh)

	cache.WaitForCacheSync(stopCh, configMapInformer.HasSynced)

	return KubeVirtClusterConfigProvider{namespace: kubevirtNamespace, configMapInformer: configMapInformer}

}

// GetConfig provides the most current KubeVirt config
func (cp *KubeVirtClusterConfigProvider) GetConfig() (KubeVirtConfig, error) {
	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
	}
	item, exists, err := cp.configMapInformer.GetStore().GetByKey(cp.namespace + "/" + configMapName)
	if err != nil {
		return KubeVirtConfig{}, err
	}
	if exists {
		configMap = *item.(*v1.ConfigMap)
	}
	return NewKubeVirtConfig(configMap), nil
}
