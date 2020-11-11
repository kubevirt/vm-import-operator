package vmware_test

import (
	"context"
	"fmt"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/client-go/api/v1"
	"time"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"

	v2vvmiclient "github.com/kubevirt/vm-import-operator/pkg/api-client/clientset/versioned/typed/v2v/v1beta1"
	oputils "github.com/kubevirt/vm-import-operator/pkg/utils"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("VM import cancellation ", func() {
	var (
		f         = framework.NewFrameworkOrDie("cancel-vm-import", framework.ProviderVmware)
		secret    corev1.Secret
		namespace string
		vmImports v2vvmiclient.VirtualMachineImportInterface
		vmi       *v2vv1.VirtualMachineImport
		vmiName   string
		err       error
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		secret, err = f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		vmImports = f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace)
		cr := utils.VirtualMachineImportCr(framework.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, true)
		vmi, err = vmImports.Create(context.TODO(), &cr, metav1.CreateOptions{})
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
			err = vmImports.Delete(context.TODO(), vmiName, deleteOptions)
			if err != nil {
				Fail(err.Error())
			}
		})

		By("Waiting for VM import removal")
		err = f.EnsureVMImportDoesNotExist(vmiName)
		Expect(err).ToNot(HaveOccurred())

		By("Temporary secret no longer existing")
		Eventually(func() (*corev1.Secret, error) {
			return getTemporarySecret(f, namespace, vmiName)
		}, 2*time.Minute, time.Second).Should(BeNil())

		By("VM Data Volume no longer existing")
		Eventually(func() error {
			_, err = f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
			return err
		}, 2*time.Minute, time.Second).Should(And(
			HaveOccurred(),
			WithTransform(errors.IsNotFound, BeTrue()),
		))

		By("VM no longer existing")
		Eventually(func() error {
			vmNamespacedName := types.NamespacedName{Namespace: namespace, Name: *vmi.Spec.TargetVMName}
			vm := &v1.VirtualMachine{}
			err = f.Client.Get(context.TODO(), vmNamespacedName, vm)
			return err
		}, 2*time.Minute, time.Second).Should(And(
			HaveOccurred(),
			WithTransform(errors.IsNotFound, BeTrue()),
		))
	})

})

func getTemporarySecret(f *framework.Framework, namespace string, vmiName string) (*corev1.Secret, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vmimport.v2v.kubevirt.io/vmi-name=%s", oputils.EnsureLabelValueLength(vmiName)),
	}
	list, err := f.K8sClient.CoreV1().Secrets(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}
