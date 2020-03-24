package validators

import (
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storage "k8s.io/client-go/kubernetes/typed/storage/v1"
)

// StorageClasses is responsible for finding storage classes
type StorageClasses struct {
	Client storage.StorageClassInterface
}

// Find retrieves storage class with provided name
func (finder *StorageClasses) Find(name string) (*v1.StorageClass, error) {
	return finder.Client.Get(name, metav1.GetOptions{})
}
