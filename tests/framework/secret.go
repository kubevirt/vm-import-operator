package framework

import (
	ovirtenv "github.com/kubevirt/vm-import-operator/tests/env/ovirt"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateOvirtSecretFromCACert creates Ovirt secret
func (f *Framework) CreateOvirtSecretFromCACert() (corev1.Secret, error) {
	return f.CreateOvirtSecretInNamespaceFromCACert(f.Namespace.Name)
}

// CreateOvirtSecretInNamespaceFromCACert creates Ovirt secret in given namespace
func (f *Framework) CreateOvirtSecretInNamespaceFromCACert(namespace string) (corev1.Secret, error) {
	return f.CreateOvirtSecretInNamespace(*ovirtenv.NewFakeOvirtEnvironment(f.ImageioInstallNamespace, f.OVirtCA), namespace)
}

// CreateOvirtSecret creates ovirt secret with given environment settings
func (f *Framework) CreateOvirtSecret(environment ovirtenv.Environment) (corev1.Secret, error) {
	return f.CreateOvirtSecretInNamespace(environment, f.Namespace.Name)
}

// CreateOvirtSecretInNamespace creates ovirt secret with given environment settings in given namespace
func (f *Framework) CreateOvirtSecretInNamespace(environment ovirtenv.Environment, namespace string) (corev1.Secret, error) {
	secretData := make(map[string]string)
	secretData["apiUrl"] = environment.ApiURL
	secretData["username"] = environment.Username
	secretData["password"] = environment.Password
	secretData["caCert"] = environment.CaCert

	marshalled, err := yaml.Marshal(secretData)
	if err != nil {
		return corev1.Secret{}, err
	}
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
