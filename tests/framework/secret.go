package framework

import (
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateOvirtSecretFromBlueprint copies secret from `f.OVirtSecretName` to the test namespace
func (f *Framework) CreateOvirtSecretFromBlueprint() (corev1.Secret, error) {
	if f.OVirtSecretName == nil {
		return corev1.Secret{}, fmt.Errorf("OVirt secret namespace and name have not been provided")
	}
	blueprint, err := f.K8sClient.CoreV1().Secrets(f.OVirtSecretName.Namespace).Get(f.OVirtSecretName.Name, metav1.GetOptions{})
	if err != nil {
		return corev1.Secret{}, err
	}
	testSecret := blueprint.DeepCopy()
	namespace := f.Namespace.Name
	testSecret.ObjectMeta = metav1.ObjectMeta{
		GenerateName: f.NsPrefix,
		Namespace:    namespace,
	}

	created, err := f.K8sClient.CoreV1().Secrets(namespace).Create(testSecret)
	if err != nil {
		return corev1.Secret{}, err
	}
	return *created, nil
}

// CreateOvirtSecret creates ovirt secret with given credentials
func (f *Framework) CreateOvirtSecret(apiURL string, username string, password string, caCert string) (corev1.Secret, error) {
	secretData := make(map[string]string)
	secretData["apiUrl"] = apiURL
	secretData["username"] = username
	secretData["password"] = password
	secretData["caCert"] = caCert

	marshalled, err := yaml.Marshal(secretData)
	if err != nil {
		return corev1.Secret{}, err
	}
	namespace := f.Namespace.Name
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.NsPrefix,
			Namespace:    namespace,
		},
		StringData: map[string]string{"ovirt": string(marshalled)},
	}
	created, err := f.K8sClient.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		return corev1.Secret{}, err
	}
	return *created, nil
}
