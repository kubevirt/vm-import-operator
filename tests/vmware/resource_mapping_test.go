package vmware_test

import (
	"context"
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/tests"
	fwk "github.com/kubevirt/vm-import-operator/tests/framework"
	. "github.com/kubevirt/vm-import-operator/tests/matchers"
	"github.com/kubevirt/vm-import-operator/tests/utils"
	"github.com/kubevirt/vm-import-operator/tests/vmware"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/client-go/api/v1"
)

var _ = Describe("VM import ", func() {

	var (
		f         = fwk.NewFrameworkOrDie("resource-mapping", fwk.ProviderVmware)
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

	Context("with external resource mapping for network", func() {
		It("should create running VM", func() {
			vmwareMappings := v2vv1.VmwareMappings{
				NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
					{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &tests.PodType},
				},
			}
			rm, err := f.CreateVmwareResourceMapping(vmwareMappings)
			if err != nil {
				Fail(err.Error())
			}

			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{Name: rm.Name, Namespace: &rm.Namespace}
			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("Having correct network configuration")
			vmNamespacedName := types.NamespacedName{Namespace: namespace, Name: vmBlueprint.Name}
			vm := &v1.VirtualMachine{}
			_ = f.Client.Get(context.TODO(), vmNamespacedName, vm)
			spec := vm.Spec.Template.Spec
			Expect(spec.Networks).To(HaveLen(1))
			Expect(spec.Networks[0].Pod).ToNot(BeNil())
			Expect(spec.Domain.Devices.Interfaces).To(HaveLen(1))
			Expect(spec.Domain.Devices.Interfaces[0].Name).To(BeEquivalentTo(spec.Networks[0].Name))

			By("Having DV with default storage class")
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveDefaultStorageClass(f))
		})
	})

	Context("with external resource mapping for datastore", func() {
		It("should create running VM", func() {
			mappings := v2vv1.VmwareMappings{
				StorageMappings: &[]v2vv1.StorageResourceMappingItem{
					{Source: v2vv1.Source{ID: &vmware.VM66Datastore}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
				},
			}
			rm, err := f.CreateVmwareResourceMapping(mappings)
			if err != nil {
				Fail(err.Error())
			}

			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM66, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{Name: rm.Name, Namespace: &rm.Namespace}
			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("Having DV with correct storage class")
			vmNamespacedName := types.NamespacedName{Namespace: namespace, Name: vmBlueprint.Name}
			vm := &v1.VirtualMachine{}
			_ = f.Client.Get(context.TODO(), vmNamespacedName, vm)
			Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(HaveStorageClass(f.DefaultStorageClass, f))
		})
	})

	Context("with both internal and external storage mappings", func() {
		table.DescribeTable("should create running VM", func(externalMapping v2vv1.VmwareMappings, internalMapping v2vv1.VmwareMappings, storageClass *string) {
			rm, err := f.CreateVmwareResourceMapping(externalMapping)
			if err != nil {
				Fail(err.Error())
			}

			vmi := utils.VirtualMachineImportCr(fwk.ProviderVmware, vmware.VM70, namespace, secret.Name, f.NsPrefix, true)
			vmi.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{Name: rm.Name, Namespace: &rm.Namespace}
			vmi.Spec.Source.Vmware.Mappings = &internalMapping
			created, err := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Create(context.TODO(), &vmi, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeSuccessful(f))

			retrieved, _ := f.VMImportClient.V2vV1beta1().VirtualMachineImports(namespace).Get(context.TODO(), created.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			vmBlueprint := v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: retrieved.Status.TargetVMName, Namespace: namespace}}
			Expect(vmBlueprint).To(BeRunning(f))

			By("having pod network from external resource mapping")
			vmNamespacedName := types.NamespacedName{Namespace: namespace, Name: vmBlueprint.Name}
			vm := &v1.VirtualMachine{}
			_ = f.Client.Get(context.TODO(), vmNamespacedName, vm)
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
				v2vv1.VmwareMappings{
					DiskMappings: &[]v2vv1.StorageResourceMappingItem{
						{Source: v2vv1.Source{Name: &vmware.VM70DiskName1}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
						{Source: v2vv1.Source{Name: &vmware.VM70DiskName2}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
					},
					NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
						{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &tests.PodType},
					},
				},
				v2vv1.VmwareMappings{},
				nil),
			table.Entry("with storage class based on datastore",
				v2vv1.VmwareMappings{
					NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
						{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &tests.PodType},
					},
				},
				v2vv1.VmwareMappings{
					StorageMappings: &[]v2vv1.StorageResourceMappingItem{
						{Source: v2vv1.Source{ID: &vmware.VM70Datastore}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
					},
				},
				&f.DefaultStorageClass),
			table.Entry("with storage class based on internal mapping overriding external mapping",
				v2vv1.VmwareMappings{
					StorageMappings: &[]v2vv1.StorageResourceMappingItem{
						{Source: v2vv1.Source{ID: &vmware.VM70Datastore}, Target: v2vv1.ObjectIdentifier{Name: "wrong-sc"}},
					},
					NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
						{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &tests.PodType},
					},
				},
				v2vv1.VmwareMappings{
					StorageMappings: &[]v2vv1.StorageResourceMappingItem{
						{Source: v2vv1.Source{Name: &vmware.VM70DatastoreName}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
					},
				},
				&f.DefaultStorageClass),
			table.Entry("with pod network based on internal network mapping overriding external mapping",
				v2vv1.VmwareMappings{
					StorageMappings: &[]v2vv1.StorageResourceMappingItem{
						{Source: v2vv1.Source{ID: &vmware.VM70Datastore}, Target: v2vv1.ObjectIdentifier{Name: f.DefaultStorageClass}},
					},
					NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
						{
							Type:   &tests.MultusType,
							Source: v2vv1.Source{Name: &vmware.VM70Network},
							Target: v2vv1.ObjectIdentifier{Name: "wrong-network"},
						},
					},
				},
				v2vv1.VmwareMappings{
					NetworkMappings: &[]v2vv1.NetworkResourceMappingItem{
						{Source: v2vv1.Source{Name: &vmware.VM70Network}, Type: &tests.PodType},
					},
				},
				&f.DefaultStorageClass),
		)
	})
})
