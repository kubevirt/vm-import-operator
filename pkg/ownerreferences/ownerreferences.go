package ownerreferences

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OwnerReferenceManager is struct that hold reference manager attributes
type OwnerReferenceManager struct {
	client client.Client
}

// NewOwnerReferenceManager create owner reference manager
func NewOwnerReferenceManager(client client.Client) OwnerReferenceManager {
	return OwnerReferenceManager{
		client: client,
	}
}

// PurgeOwnerReferences cleans the owner references of the Virtual Machine.
func (m *OwnerReferenceManager) PurgeOwnerReferences(vmName types.NamespacedName) []error {
	var errs []error

	vm := kubevirtv1.VirtualMachine{}
	err := m.client.Get(context.TODO(), types.NamespacedName{Namespace: vmName.Namespace, Name: vmName.Name}, &vm)
	if err != nil {
		errs = append(errs, err)
		// Stop here - we can't process further without a VM
		return errs
	}

	e := m.removeDataVolumesOwnerReferences(&vm)
	if len(e) > 0 {
		errs = append(errs, e...)
	}
	err = m.removeVMOwnerReference(&vm)
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

// AddOwnerReference add owner refence of the Virtual Machine to the DataVolume
func (m *OwnerReferenceManager) AddOwnerReference(vm *kubevirtv1.VirtualMachine, dv *cdiv1.DataVolume) error {
	ownerRefs := dv.GetOwnerReferences()
	if ownerRefs == nil {
		ownerRefs = []metav1.OwnerReference{}
	}
	ownerRefs = append(ownerRefs, newVMOwnerReference(vm))
	dvCopy := dv.DeepCopy()
	dvCopy.SetOwnerReferences(ownerRefs)
	patch := client.MergeFrom(dv)
	return m.client.Patch(context.TODO(), dvCopy, patch)
}

func (m *OwnerReferenceManager) removeDataVolumesOwnerReferences(vm *kubevirtv1.VirtualMachine) []error {
	var errs []error
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if v.DataVolume != nil {
			err := m.removeDataVolumeOwnerReference(vm.Namespace, v.DataVolume.Name)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (m *OwnerReferenceManager) removeVMOwnerReference(vm *kubevirtv1.VirtualMachine) error {
	refs := vm.GetOwnerReferences()
	newRefs := removeControllerReference(refs)
	if len(newRefs) < len(refs) {
		vmCopy := vm.DeepCopy()
		vmCopy.SetOwnerReferences(newRefs)
		patch := client.MergeFrom(vm)
		return m.client.Patch(context.TODO(), vmCopy, patch)
	}
	return nil
}

func (m *OwnerReferenceManager) removeDataVolumeOwnerReference(namespace string, dvName string) error {
	dv := &cdiv1.DataVolume{}
	err := m.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: dvName}, dv)
	if err != nil {
		return err
	}

	refs := dv.GetOwnerReferences()
	newRefs := removeControllerReference(refs)
	if len(newRefs) < len(refs) {
		dvCopy := dv.DeepCopy()
		dvCopy.SetOwnerReferences(newRefs)
		patch := client.MergeFrom(dv)
		return m.client.Patch(context.TODO(), dvCopy, patch)
	}
	return nil
}

func removeControllerReference(refs []metav1.OwnerReference) []metav1.OwnerReference {
	for i := range refs {
		isController := refs[i].Controller
		if isController != nil && *isController {
			// There can be only one controller reference
			return append(refs[:i], refs[i+1:]...)
		}
	}
	return refs
}

func newVMOwnerReference(vm *kubevirtv1.VirtualMachine) metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := false
	return metav1.OwnerReference{
		APIVersion:         vm.GroupVersionKind().GroupVersion().String(),
		Kind:               vm.GetObjectKind().GroupVersionKind().Kind,
		Name:               vm.GetName(),
		UID:                vm.GetUID(),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}

// NewVMImportOwnerReference create a new Ownerrefercen based on passed parameters
func NewVMImportOwnerReference(typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta) metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := false
	return metav1.OwnerReference{
		APIVersion:         typeMeta.APIVersion,
		Kind:               typeMeta.GetObjectKind().GroupVersionKind().Kind,
		Name:               objectMeta.GetName(),
		UID:                objectMeta.GetUID(),
		Controller:         &isController,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}
