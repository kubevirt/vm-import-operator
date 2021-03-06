package ovirt_test

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	"github.com/kubevirt/vm-import-operator/tests/ovirt/vms"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"

	v2vvmiclient "github.com/kubevirt/vm-import-operator/pkg/api-client/clientset/versioned/typed/v2v/v1beta1"
	oputils "github.com/kubevirt/vm-import-operator/pkg/utils"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	sapi "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type targetVmDeletionTest struct {
	framework *framework.Framework
}

var _ = Describe("Target VM deletion ", func() {
	var (
		f         = framework.NewFrameworkOrDie("deleted-vm-import", framework.ProviderOvirt)
		secret    corev1.Secret
		namespace string
		vmImports v2vvmiclient.VirtualMachineImportInterface
		vmi       *v2vv1.VirtualMachineImport
		vmiName   string
		test      = targetVmDeletionTest{framework: f}
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromCACert()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
		vmImports = f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace)
		vmID := vms.BasicVmID
		cr := utils.VirtualMachineImportCr(framework.ProviderOvirt, vmID, namespace, secret.Name, f.NsPrefix, true)
		test.stub(f, vmID)
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

		targetVMName := types.NamespacedName{Namespace: vmi.Namespace, Name: *vmi.Spec.TargetVMName}

		When("Target VM is deleted in the foreground", func() {
			foreground := client.PropagationPolicy(metav1.DeletePropagationForeground)

			vm := &kubevirtv1.VirtualMachine{}
			vm.Name = targetVMName.Name
			vm.Namespace = targetVMName.Namespace

			err = f.Client.Delete(context.TODO(), vm, foreground)
			if err != nil {
				Fail(err.Error())
			}
		})

		By("Waiting for VM removal")
		err = f.EnsureVMDoesNotExist(vmiName)
		Expect(err).ToNot(HaveOccurred())

		By("Temporary config map no longer existing")
		Eventually(func() (*corev1.ConfigMap, error) {
			return test.getTemporaryConfigMap(f, namespace, vmiName)
		}, 2*time.Minute, time.Second).Should(BeNil())

		By("Temporary secret no longer existing")
		Eventually(func() (*corev1.Secret, error) {
			return test.getTemporarySecret(f, namespace, vmiName)
		}, 2*time.Minute, time.Second).Should(BeNil())

		By("VM Data Volume no longer existing")
		Eventually(func() error {
			_, err = f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
			return err
		}, 2*time.Minute, time.Second).Should(And(
			HaveOccurred(),
			WithTransform(errors.IsNotFound, BeTrue()),
		))
	})
})

func (t *targetVmDeletionTest) getTemporaryConfigMap(f *framework.Framework, namespace string, vmiName string) (*corev1.ConfigMap, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vmimport.v2v.kubevirt.io/vmi-name=%s", oputils.EnsureLabelValueLength(vmiName)),
	}
	list, err := f.K8sClient.CoreV1().ConfigMaps(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

func (t *targetVmDeletionTest) getTemporarySecret(f *framework.Framework, namespace string, vmiName string) (*corev1.Secret, error) {
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

func (t *targetVmDeletionTest) stub(f *framework.Framework, vmID string) {
	domainXml := f.LoadFile("storage-domains/domain-1.xml")
	diskAttachmentsXml := f.LoadFile("disk-attachments/one.xml")
	diskXml := f.LoadTemplate("disks/disk-1.xml", map[string]string{"@DISKSIZE": "50331648"})
	consolesXml := f.LoadFile("graphic-consoles/vnc.xml")
	vmXml := f.LoadTemplate("vms/basic-vm.xml", map[string]string{"@VMID": vmID})
	nicsXml := f.LoadFile("nics/empty.xml")
	builder := sapi.NewStubbingBuilder().
		StubGet("/ovirt-engine/api/vms/"+vmID+"/diskattachments", &diskAttachmentsXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/graphicsconsoles", &consolesXml).
		StubGet("/ovirt-engine/api/vms/"+vmID+"/nics", &nicsXml).
		StubGet("/ovirt-engine/api/disks/disk-1", &diskXml).
		StubGet("/ovirt-engine/api/storagedomains/domain-1", &domainXml).
		StubGet("/ovirt-engine/api/vms/"+vmID, &vmXml)
	err := f.OvirtStubbingClient.Stub(builder.Build())
	if err != nil {
		Fail(err.Error())
	}
}
