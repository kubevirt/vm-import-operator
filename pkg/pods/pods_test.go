package pods

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	deleteCalled = false
)

var _ = Describe("Pods Manager", func() {

	manager := NewManager(mockClient{})
	vmiCrName := types.NamespacedName{Name: "test", Namespace: "test"}

	Describe("FindFor", func() {
		It("should return nil, nil when there are no pods found", func() {
			list = func(context.Context, runtime.Object) error {
				return nil
			}
			pod, err := manager.FindFor(vmiCrName)
			Expect(pod).To(BeNil())
			Expect(err).To(BeNil())
		})

		It("should return an error if more than one pod is found", func() {
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*corev1.PodList).Items = []corev1.Pod{
					{}, {},
				}
				return nil
			}
			pod, err := manager.FindFor(vmiCrName)
			Expect(pod).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("too many pods matching given labels: map[vmimport.v2v.kubevirt.io/vmi-name:test]"))
		})

		It("should return the pod if one is found", func() {
			testPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
			}
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*corev1.PodList).Items = []corev1.Pod{
					testPod,
				}
				return nil
			}
			pod, err := manager.FindFor(vmiCrName)
			Expect(pod).ToNot(BeNil())
			Expect(err).To(BeNil())
			Expect(pod.Name).To(Equal(testPod.Name))
		})
	})

	Describe("CreateFor", func() {
		It("should set GenerateName, blank out Name, and set the vmi-name label", func() {
			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "should-be-blanked",
				},
			}

			err := manager.CreateFor(testPod, vmiCrName)
			Expect(err).To(BeNil())
			Expect(testPod.GenerateName).To(Equal(prefix))
			Expect(testPod.Name).To(Equal(""))
			Expect(testPod.Labels[vmiNameLabel]).To(Equal(vmiCrName.Name))
		})
	})

	Describe("DeleteFor", func() {
		BeforeEach(func() {
			deleteCalled = false
		})

		It("should return nil when there are no pods found", func() {
			list = func(context.Context, runtime.Object) error {
				return nil
			}
			err := manager.DeleteFor(vmiCrName)
			Expect(deleteCalled).To(BeFalse())
			Expect(err).To(BeNil())
		})

		It("should return an error if more than one pod is found", func() {
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*corev1.PodList).Items = []corev1.Pod{
					{}, {},
				}
				return nil
			}
			err := manager.DeleteFor(vmiCrName)
			Expect(deleteCalled).To(BeFalse())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("too many pods matching given labels: map[vmimport.v2v.kubevirt.io/vmi-name:test]"))
		})

		It("should return nil if one pod is found", func() {
			testPod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
			}
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*corev1.PodList).Items = []corev1.Pod{
					testPod,
				}
				return nil
			}
			err := manager.DeleteFor(vmiCrName)
			Expect(deleteCalled).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})
})

type mockClient struct{}

var list func(context.Context, runtime.Object) error

// Create implements client.Client
func (c mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return nil
}

// Update implements client.Client
func (c mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

// Delete implements client.Client
func (c mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	deleteCalled = true
	return nil
}

// DeleteAllOf implements client.Client
func (c mockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

// Patch implements client.Client
func (c mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

// Get implements client.Client
func (c mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return nil
}

// List implements client.Client
func (c mockClient) List(ctx context.Context, objectList runtime.Object, opts ...client.ListOption) error {
	return list(ctx, objectList)
}

// Status implements client.StatusClient
func (c mockClient) Status() client.StatusWriter {
	return c
}
