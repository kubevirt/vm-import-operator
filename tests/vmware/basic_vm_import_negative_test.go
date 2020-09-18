package vmware_test

import (
	"github.com/kubevirt/vm-import-operator/tests"
	env "github.com/kubevirt/vm-import-operator/tests/env/vmware"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
	"strings"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type basicVMImportNegativeTest struct {
	framework *fwk.Framework
}

var _ = Describe("VM import", func() {

	var (
		f         = fwk.NewFrameworkOrDie("basic-vm-import-negative", fwk.ProviderVmware)
		secret    corev1.Secret
		namespace string
		test      = basicVMImportNegativeTest{f}
		err       error
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name

		secret, err = f.CreateVmwareSecretInNamespace(namespace)
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
	})

	It("should fail for missing secret", func() {
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, "no-such-secret", f.NsPrefix, true)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.SecretNotFound)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	})

	It("should fail for invalid secret", func() {
		invalidSecret := test.createInvalidSecret()
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, invalidSecret.Name, f.NsPrefix, true)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.UninitializedProvider)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	})

	table.DescribeTable("should fail for invalid ", func(env *env.Environment) {
		invalidSecret, err := f.CreateVmwareSecret(*env, namespace)
		if err != nil {
			Fail(err.Error())
		}
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, invalidSecret.Name, f.NsPrefix, true)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.UninitializedProvider)))
		Expect(created).To(BeUnsuccessful(f, string(v2vv1.ValidationFailed)))
		retrieved, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
		Expect(retrieved.Status.Conditions).To(HaveLen(2))
	},
		table.Entry("vSphere URL", env.NewVcsimEnvironment(f.VcsimInstallNamespace).WithAPIURL("")),
		table.Entry("vSphere username", env.NewVcsimEnvironment(f.VcsimInstallNamespace).WithUsername("")),
		table.Entry("vSphere password", env.NewVcsimEnvironment(f.VcsimInstallNamespace).WithPassword("")),
	)

	It("should fail for non-existing VM ID", func() {
		vmID := "does-not-exist"
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmID, namespace, secret.Name, f.NsPrefix, true)

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.SourceVMNotFound)))
	})

	It("should fail for missing specified external mapping", func() {
		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{Name: "does-not-exist", Namespace: &namespace}

		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveValidationFailure(f, string(v2vv1.ResourceMappingNotFound)))
	})

	It("should fail when ImportWithoutTemplate feature gate is disabled and VM template can't be found", func() {
		configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get("kubevirt-config", metav1.GetOptions{})
		if err != nil {
			Fail(err.Error())
		}
		configMap.Data["feature-gates"] = strings.ReplaceAll(configMap.Data["feature-gates"], ",ImportWithoutTemplate", "")

		f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Update(configMap)
		defer test.cleanUpConfigMap()

		vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true)
		vmi.Spec.Source.Vmware.Mappings = &v2vv1.VmwareMappings{
			NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
				{Source: v2vv1.Source{ID: &vmware.VM70Network}, Type: &tests.PodType},
			},
		}
		created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(HaveTemplateMatchingFailure(f))
	})

	It("should fail when targetVMName is too long", func() {
		vmName := strings.Repeat("x", 64)
		vmi := utils.VirtualMachineImportCrWithName(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true, vmName)

		_, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(&vmi)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.targetVmName in body should be at most 63 chars long"))
	})
})

func (t *basicVMImportNegativeTest) createInvalidSecret() *corev1.Secret {
	f := t.framework
	namespace := f.Namespace.Name
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.NsPrefix,
			Namespace:    namespace,
		},
		StringData: map[string]string{"vmware": "garbage"},
	}
	created, err := f.K8sClient.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		Fail(err.Error())
	}
	return created
}

func (t *basicVMImportNegativeTest) cleanUpConfigMap() {
	f := t.framework
	configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Get("kubevirt-config", metav1.GetOptions{})
	if err != nil {
		Fail(err.Error())
	}
	configMap.Data["feature-gates"] = configMap.Data["feature-gates"] + ",ImportWithoutTemplate"
	_, err = f.K8sClient.CoreV1().ConfigMaps(f.KubeVirtInstallNamespace).Update(configMap)
	if err != nil {
		Fail(err.Error())
	}
}
