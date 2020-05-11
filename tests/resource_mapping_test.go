package tests_test

import (
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	vms "github.com/kubevirt/vm-import-operator/tests/ovirt-vms"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("VM import ", func() {

	var (
		f         = framework.NewFrameworkOrDie("resource-mapping")
		secret    corev1.Secret
		namespace string
	)

	BeforeEach(func() {
		namespace = f.Namespace.Name
		s, err := f.CreateOvirtSecretFromBlueprint()
		if err != nil {
			Fail("Cannot create secret: " + err.Error())
		}
		secret = s
	})

	Context("with external resource mapping for network", func() {
		It("should create running VM", func() {
			ovirtMappings := v2vv1alpha1.OvirtMappings{
				NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &podType},
				},
			}
			rm, err := f.CreateResourceMapping(ovirtMappings)
			if err != nil {
				Fail(err.Error())
			}
			vmi := utils.VirtualMachineImportCr(vms.BasicNetworkVmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{Name: rm.Name, Namespace: &rm.Namespace}
			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("Having correct network configuration")
			vm, _ := f.KubeVirtClient.VirtualMachine(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			spec := vm.Spec.Template.Spec
			Expect(spec.Networks).To(HaveLen(1))
			Expect(spec.Networks[0].Pod).ToNot(BeNil())
			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(1))
			Expect(spec.Domain.Devices.Interfaces[0].Name).To(BeEquivalentTo(spec.Networks[0].Name))

			By("Having DV with default storage class")
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))
		})
	})

	Context("with external resource mapping for storage domain", func() {
		It("should create running VM", func() {
			mappings := v2vv1alpha1.OvirtMappings{
				StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{
					{Source: v2vv1alpha1.Source{ID: &vms.StorageDomainID}, Target: v2vv1alpha1.ObjectIdentifier{Name: storageClass}},
				},
			}
			rm, err := f.CreateResourceMapping(mappings)
			if err != nil {
				Fail(err.Error())
			}
			vmi := utils.VirtualMachineImportCr(vms.BasicVmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{Name: rm.Name, Namespace: &rm.Namespace}
			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("Having DV with correct storage class")
			vm, _ := f.KubeVirtClient.VirtualMachine(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveStorageClass(storageClass, f))
		})
	})

	Context("with both internal and external storage mappings", func() {
		table.DescribeTable("should create running VM", func(externalMapping v2vv1alpha1.OvirtMappings, internalMapping v2vv1alpha1.OvirtMappings, storageClass *string) {
			rm, err := f.CreateResourceMapping(externalMapping)
			if err != nil {
				Fail(err.Error())
			}
			vmi := utils.VirtualMachineImportCr(vms.BasicNetworkVmID, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{Name: rm.Name, Namespace: &rm.Namespace}
			vmi.Spec.Source.Ovirt.Mappings = &internalMapping
			created, err := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Create(&vmi)

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1alpha1().VirtualMachineImports(namespace).Get(created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("having pod network from external resource mapping")
			vm, _ := f.KubeVirtClient.VirtualMachine(namespace).Get(vmBlueprint.Name, &metav1.GetOptions{})
			spec := vm.Spec.Template.Spec
			Expect(spec.Networks).To(HaveLen(1))
			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(1))
			Expect(spec.Domain.Devices.Interfaces[0].Name).To(BeEquivalentTo(spec.Networks[0].Name))
			Expect(spec.Networks[0].Pod).ToNot(BeNil())
			Expect(spec.Networks[0].Multus).To(BeNil())

			By("Having DV with correct storage class")
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveStorageClassReference(storageClass, f))
		},
			table.Entry("with default storage class ignoring external mapping",
				v2vv1alpha1.OvirtMappings{
					DiskMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.VirtioDiskID}, Target: v2vv1alpha1.ObjectIdentifier{Name: storageClass}},
					},
					NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &podType},
					},
				},
				v2vv1alpha1.OvirtMappings{},
				nil),
			table.Entry("with storage class based on internal storage domain",
				v2vv1alpha1.OvirtMappings{
					NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &podType},
					},
				},
				v2vv1alpha1.OvirtMappings{
					StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.StorageDomainID}, Target: v2vv1alpha1.ObjectIdentifier{Name: storageClass}},
					},
				},
				&storageClass),
			table.Entry("with storage class based on internal mapping overriding external mapping",
				v2vv1alpha1.OvirtMappings{
					StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.StorageDomainID}, Target: v2vv1alpha1.ObjectIdentifier{Name: "wrong-sc"}},
					},
					NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &podType},
					},
				},
				v2vv1alpha1.OvirtMappings{
					StorageMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.StorageDomainID}, Target: v2vv1alpha1.ObjectIdentifier{Name: storageClass}},
					},
				},
				&storageClass),
			table.Entry("with pod network based on internal network mapping overriding external mapping",
				v2vv1alpha1.OvirtMappings{
					NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{
							Type:   &multusType,
							Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID},
							Target: v2vv1alpha1.ObjectIdentifier{Name: "wrong-network"},
						},
					},
				},
				v2vv1alpha1.OvirtMappings{
					NetworkMappings: &[]v2vv1alpha1.ResourceMappingItem{
						{Source: v2vv1alpha1.Source{ID: &vms.BasicNetworkID}, Type: &podType},
					},
				},
				nil),
		)
	})
})
