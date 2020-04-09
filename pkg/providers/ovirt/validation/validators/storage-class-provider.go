package validators

import (
	"context"

	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StorageClasses is responsible for finding storage classes
type StorageClasses struct {
	Client client.Client
}

// Find retrieves storage class with provided name
func (finder *StorageClasses) Find(name string) (*v1.StorageClass, error) {
	sc := &v1.StorageClass{}
	err := finder.Client.Get(context.TODO(), types.NamespacedName{Name: name}, sc)
	return sc, err
}
