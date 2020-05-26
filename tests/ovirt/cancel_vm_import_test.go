package ovirt_test

import (
	"fmt"
	"time"

	vms2 "github.com/kubevirt/vm-import-operator/tests/ovirt/vms"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"

	v2vvmiclient "github.com/kubevirt/vm-import-operator/pkg/api-client/clientset/versioned/typed/v2v/v1alpha1"
	oputils "github.com/kubevirt/vm-import-operator/pkg/utils"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("VM import cancellation ", func() {
	var (
		f         = framework.NewFrameworkOrDie("cancel-vm-import")
		secret    corev1.Secret
		namespace string
		vmImports v2vvmiclient.VirtualMachineImportInterface
		vmi       *v2vv1alpha1.VirtualMachineImport
		vmiName   string
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
		vmImports = f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace)
		cr := utils.VirtualMachineImportCr(vms2.BasicVmID, namespace, secret.Name, f.NsPrefix, true)
		vmi, err = vmImports.Create(&cr)
		if err != nil {
			Fail(err.Error())
		}
		vmiName = vmi.Name
		err = f.WaitForVMToBeProcessing(vmiName)
		if err != nil {
			Fail(err.Error())
		}
	})

	It("should have deleted all the import-associated resources", func() {
		By("Temporary config map existing - sanity check")
		configMap, err := getTemporaryConfigMap(f, namespace, vmiName)
		Expect(err).ToNot(HaveOccurred())
		Expect(configMap).ToNot(BeNil())

		By("Temporary secret existing - sanity check")
		secret, err := getTemporarySecret(f, namespace, vmiName)
		Expect(err).ToNot(HaveOccurred())
		Expect(secret).ToNot(BeNil())

		By("Virtual Machine existing - sanity check")
		vm, err := f.WaitForVMToExist(*vmi.Spec.TargetVMName)
		Expect(err).ToNot(HaveOccurred())

		dvName := vm.Spec.Template.Spec.Volumes[0].DataVolume.Name

		By("Data Volume existing - sanity check")
		err = f.WaitForDataVolumeToExist(dvName)
		Expect(err).NotTo(HaveOccurred())

		When("VM Import is deleted in the foreground", func() {
			foreground := metav1.DeletePropagationForeground
			deleteOptions := metav1.DeleteOptions{
				PropagationPolicy: &foreground,
			}
			err = vmImports.Delete(vmiName, &deleteOptions)
			if err != nil {
				Fail(err.Error())
			}
		})

		By("Waiting for VM import removal")
		err = f.EnsureVMIDoesNotExist(vmiName)
		Expect(err).ToNot(HaveOccurred())

		By("Temporary config map no longer existing")
		Eventually(func() (*corev1.ConfigMap, error) {
			return getTemporaryConfigMap(f, namespace, vmiName)
		}, 2*time.Minute, time.Second).Should(BeNil())

		By("Temporary secret no longer existing")
		Eventually(func() (*corev1.Secret, error) {
			return getTemporarySecret(f, namespace, vmiName)
		}, 2*time.Minute, time.Second).Should(BeNil())

		By("VM Data Volume no longer existing")
		Eventually(func() error {
			_, err = f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dvName, metav1.GetOptions{})
			return err
		}, 2*time.Minute, time.Second).Should(And(
			HaveOccurred(),
			WithTransform(errors.IsNotFound, BeTrue()),
		))

		By("VM no longer existing")
		Eventually(func() error {
			_, err = f.KubeVirtClient.VirtualMachine(namespace).Get(*vmi.Spec.TargetVMName, &metav1.GetOptions{})
			return err
		}, 2*time.Minute, time.Second).Should(And(
			HaveOccurred(),
			WithTransform(errors.IsNotFound, BeTrue()),
		))
	})

})

func getTemporaryConfigMap(f *framework.Framework, namespace string, vmiName string) (*corev1.ConfigMap, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vmimport.v2v.kubevirt.io/vmi-name=%s", oputils.MakeLabelFrom(vmiName)),
	}
	list, err := f.K8sClient.CoreV1().ConfigMaps(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

func getTemporarySecret(f *framework.Framework, namespace string, vmiName string) (*corev1.Secret, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vmimport.v2v.kubevirt.io/vmi-name=%s", oputils.MakeLabelFrom(vmiName)),
	}
	list, err := f.K8sClient.CoreV1().Secrets(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}
