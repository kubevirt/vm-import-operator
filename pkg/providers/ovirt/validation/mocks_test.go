package validation

import (
	"context"

	validators "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var clientGetMock func(runtime.Object) error

var statusUpdateObjects []runtime.Object

var validateVMMock func(*ovirtsdk.Vm) []validators.ValidationFailure
var validateNicsMock func([]*ovirtsdk.Nic) []validators.ValidationFailure
var validateDiskAttachmentsMock func([]*ovirtsdk.DiskAttachment) []validators.ValidationFailure

type mockValidator struct{}

func (v *mockValidator) ValidateVM(vm *ovirtsdk.Vm) []validators.ValidationFailure {
	return validateVMMock(vm)
}

func (v *mockValidator) ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
	return validateDiskAttachmentsMock(diskAttachments)
}

func (v *mockValidator) ValidateNics(nics []*ovirtsdk.Nic) []validators.ValidationFailure {
	return validateNicsMock(nics)
}

type mockClient struct {
	statusWriter mockStatusWriter
}

func (c mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return clientGetMock(obj)
}

func (c mockClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	return nil
}

func (c mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return nil
}

func (c mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return nil
}

func (c mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

func (c mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (c mockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}
func (c mockClient) Status() client.StatusWriter {
	return c.statusWriter
}

type mockStatusWriter struct {
}

func (c mockStatusWriter) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	statusUpdateObjects = append(statusUpdateObjects, obj)
	return nil
}

func (c mockStatusWriter) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}
