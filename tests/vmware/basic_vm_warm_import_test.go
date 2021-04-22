package vmware_test

import (
	"context"
	"github.com/kubevirt/vm-import-operator/tests"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
	"time"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Basic VM warm import ", func() {

	var (
		f         = fwk.NewFrameworkOrDie("basic-vm-warm-import", fwk.ProviderVmware)
		secret    corev1.Secret
		namespace string
		err       error
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name

		secret, err = f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
	})

	It("should finalize a warm import", func() {
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, false)
		vmi.Spec.Source.Vmware.Mappings = &v2vv1.VmwareMappings{
			NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{Name: &vmware.VM66Network}, Type: &tests.PodType},
			},
		}
		vmi.Spec.Warm = true
		finalize := metav1.NewTime(time.Now().Add(time.Duration(2) * time.Minute))
		vmi.Spec.FinalizeDate = &finalize

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeProcessingWithReason(f, string(v2vv1.CopyingStage)))
		Expect(created.Status.WarmImport.Successes).To(Equal(0))

		paused, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(paused).To(BeProcessingWithReason(f, string(v2vv1.CopyingPaused)))

		finalized, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(finalized).To(BeSuccessful(f))
	})
})
