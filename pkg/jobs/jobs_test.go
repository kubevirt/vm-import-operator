package jobs

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	deleteCalled = false
)

var _ = Describe("Jobs Manager", func() {

	manager := NewManager(mockClient{})
	vmiCrName := types.NamespacedName{Name: "test", Namespace: "test"}

	Describe("FindFor", func() {
		It("should return nil, nil when there are no jobs found", func() {
			list = func(context.Context, runtime.Object) error {
				return nil
			}
			job, err := manager.FindFor(vmiCrName)
			Expect(job).To(BeNil())
			Expect(err).To(BeNil())
		})

		It("should return an error if more than one job is found", func() {
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*batchv1.JobList).Items = []batchv1.Job{
					{}, {},
				}
				return nil
			}
			job, err := manager.FindFor(vmiCrName)
			Expect(job).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("too many jobs matching given labels: map[vmimport.v2v.kubevirt.io/vmi-name:test]"))
		})

		It("should return the job if one is found", func() {
			testJob := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-job",
				},
			}
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*batchv1.JobList).Items = []batchv1.Job{
					testJob,
				}
				return nil
			}
			job, err := manager.FindFor(vmiCrName)
			Expect(job).ToNot(BeNil())
			Expect(err).To(BeNil())
			Expect(job.Name).To(Equal(testJob.Name))
		})
	})

	Describe("CreateFor", func() {
		It("should set GenerateName, blank out Name, and set the vmi-name label", func() {
			testJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "should-be-blanked",
				},
			}

			err := manager.CreateFor(testJob, vmiCrName)
			Expect(err).To(BeNil())
			Expect(testJob.GenerateName).To(Equal(prefix))
			Expect(testJob.Name).To(Equal(""))
			Expect(testJob.Labels[vmiNameLabel]).To(Equal(vmiCrName.Name))
		})
	})

	Describe("DeleteFor", func() {
		BeforeEach(func() {
			deleteCalled = false
		})

		It("should return nil when there are no jobs found", func() {
			list = func(context.Context, runtime.Object) error {
				return nil
			}
			err := manager.DeleteFor(vmiCrName)
			Expect(deleteCalled).To(BeFalse())
			Expect(err).To(BeNil())
		})

		It("should return an error if more than one job is found", func() {
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*batchv1.JobList).Items = []batchv1.Job{
					{}, {},
				}
				return nil
			}
			err := manager.DeleteFor(vmiCrName)
			Expect(deleteCalled).To(BeFalse())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("too many jobs matching given labels: map[vmimport.v2v.kubevirt.io/vmi-name:test]"))
		})

		It("should return nil if one job is found", func() {
			testJob := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-job",
				},
			}
			list = func(_ context.Context, obj runtime.Object) error {
				obj.(*batchv1.JobList).Items = []batchv1.Job{
					testJob,
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
