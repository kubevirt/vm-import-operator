package framework

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateResourceMapping creates resource mapping with given oVirt Mappings
func (f *Framework) CreateResourceMapping(ovirtMappings v2vv1alpha1.OvirtMappings) (v2vv1alpha1.ResourceMapping, error) {
	resourceMapping := v2vv1alpha1.ResourceMapping{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.NsPrefix,
			Namespace:    f.Namespace.Name,
		},
		Spec: v2vv1alpha1.ResourceMappingSpec{OvirtMappings: &ovirtMappings},
	}
	rm, err := f.VMImportClient.V2vV1alpha1().ResourceMappings(f.Namespace.Name).Create(&resourceMapping)
	if err != nil {
		return v2vv1alpha1.ResourceMapping{}, err
	}
	return *rm, err
}
