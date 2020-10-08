package framework

import (
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateOvirtResourceMapping creates resource mapping with given oVirt Mappings
func (f *Framework) CreateOvirtResourceMapping(ovirtMappings v2vv1.OvirtMappings) (v2vv1.ResourceMapping, error) {
	resourceMapping := v2vv1.ResourceMapping{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.NsPrefix,
			Namespace:    f.Namespace.Name,
		},
		Spec: v2vv1.ResourceMappingSpec{OvirtMappings: &ovirtMappings},
	}
	resourceMapping, err := f.createResourceMapping(resourceMapping)
	if err != nil {
		return v2vv1.ResourceMapping{}, err
	}
	return resourceMapping, err
}

// CreateVmwareResourceMapping creates resource mapping with given VmwareMappings
func (f *Framework) CreateVmwareResourceMapping(vmwareMappings v2vv1.VmwareMappings) (v2vv1.ResourceMapping, error) {
	resourceMapping := v2vv1.ResourceMapping{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.NsPrefix,
			Namespace:    f.Namespace.Name,
		},
		Spec: v2vv1.ResourceMappingSpec{VmwareMappings: &vmwareMappings},
	}
	resourceMapping, err := f.createResourceMapping(resourceMapping)
	if err != nil {
		return v2vv1.ResourceMapping{}, err
	}
	return resourceMapping, err
}

func (f *Framework) createResourceMapping(resourceMapping v2vv1.ResourceMapping) (v2vv1.ResourceMapping, error) {
	rm, err := f.VMImportClient.V2vV1beta1().ResourceMappings(f.Namespace.Name).Create(&resourceMapping)
	if err != nil {
		return v2vv1.ResourceMapping{}, err
	}
	return *rm, err
}