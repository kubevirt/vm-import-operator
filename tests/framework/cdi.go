package framework

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitForDataVolumeToExist blocks until Data Volume is created
func (f *Framework) WaitForDataVolumeToExist(dvName string) error {
	pollErr := wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
		_, err := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dvName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return pollErr
}
