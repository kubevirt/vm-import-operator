package conditions_test

import (
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Condition management", func() {
	It("should find condition by type", func() {
		validating := v2vv1alpha1.VirtualMachineImportCondition{
			Type: v2vv1alpha1.Validating,
		}
		processing := v2vv1alpha1.VirtualMachineImportCondition{
			Type: v2vv1alpha1.Processing,
		}
		vmiConditions := []v2vv1alpha1.VirtualMachineImportCondition{
			validating,
			processing,
		}

		foundValidating := conditions.FindConditionOfType(vmiConditions, validating.Type)
		foundProcessing := conditions.FindConditionOfType(vmiConditions, processing.Type)

		Expect(*foundValidating).To(Equal(validating))
		Expect(*foundProcessing).To(Equal(processing))
	})
	It("should not find condition by type when it doesn't exist", func() {
		validating := v2vv1alpha1.VirtualMachineImportCondition{
			Type: v2vv1alpha1.Validating,
		}
		processing := v2vv1alpha1.VirtualMachineImportCondition{
			Type: v2vv1alpha1.Processing,
		}
		vmiConditions := []v2vv1alpha1.VirtualMachineImportCondition{
			validating,
			processing,
		}

		found := conditions.FindConditionOfType(vmiConditions, v2vv1alpha1.MappingRulesChecking)

		Expect(found).To(BeNil())
	})
	It("should add condition", func() {
		validating := v2vv1alpha1.VirtualMachineImportCondition{
			Type: v2vv1alpha1.Validating,
		}
		vmi := v2vv1alpha1.VirtualMachineImport{
			Status: v2vv1alpha1.VirtualMachineImportStatus{
				Conditions: []v2vv1alpha1.VirtualMachineImportCondition{
					validating,
				},
			},
		}

		message := "message"
		reason := "reason"
		status := v1.ConditionTrue
		now := metav1.NewTime(time.Now())
		newCondition := v2vv1alpha1.VirtualMachineImportCondition{
			Type:               v2vv1alpha1.Processing,
			Message:            &message,
			Reason:             &reason,
			Status:             status,
			LastHeartbeatTime:  &now,
			LastTransitionTime: &now,
		}

		conditions.UpsertCondition(&vmi, newCondition)

		updatedConditions := vmi.Status.Conditions
		Expect(updatedConditions).To(HaveLen(2))
		foundValidating := conditions.FindConditionOfType(updatedConditions, v2vv1alpha1.Validating)
		Expect(*foundValidating).To(Equal(validating))

		foundProcessing := conditions.FindConditionOfType(updatedConditions, v2vv1alpha1.Processing)
		Expect(*foundProcessing.Message).To(Equal(message))
		Expect(*foundProcessing.Reason).To(Equal(reason))
		Expect(foundProcessing.Status).To(Equal(status))
		Expect(foundProcessing.LastHeartbeatTime.Time).To(BeTemporally("<=", time.Now()))
		Expect(foundProcessing.LastTransitionTime.Time).To(BeTemporally("<=", time.Now()))

	})
	It("should update condition", func() {
		oldMessage := "old-message"
		oldReason := "old-reason"
		minuteAgo := metav1.NewTime(time.Now().Add(-time.Minute))
		beforeUpdate := v2vv1alpha1.VirtualMachineImportCondition{
			Type:               v2vv1alpha1.Validating,
			Message:            &oldMessage,
			Reason:             &oldReason,
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  &minuteAgo,
			LastTransitionTime: &minuteAgo,
		}
		vmi := v2vv1alpha1.VirtualMachineImport{
			Status: v2vv1alpha1.VirtualMachineImportStatus{
				Conditions: []v2vv1alpha1.VirtualMachineImportCondition{
					beforeUpdate,
				},
			},
		}

		message := "message"
		reason := "reason"
		status := v1.ConditionTrue
		now := metav1.NewTime(time.Now())
		newCondition := v2vv1alpha1.VirtualMachineImportCondition{
			Type:               v2vv1alpha1.Validating,
			Message:            &message,
			Reason:             &reason,
			Status:             status,
			LastHeartbeatTime:  &now,
			LastTransitionTime: &now,
		}

		conditions.UpsertCondition(&vmi, newCondition)

		updatedConditions := vmi.Status.Conditions
		Expect(updatedConditions).To(HaveLen(1))
		found := conditions.FindConditionOfType(updatedConditions, v2vv1alpha1.Validating)
		Expect(*found.Message).To(Equal(message))
		Expect(*found.Reason).To(Equal(reason))
		Expect(found.Status).To(Equal(status))
		Expect(found.LastHeartbeatTime.Time).To(BeTemporally("<=", time.Now()))
		Expect(found.LastTransitionTime.Time).To(BeTemporally("<=", time.Now()))
		Expect(found.LastHeartbeatTime.Time).To(BeTemporally(">", beforeUpdate.LastHeartbeatTime.Time))
		Expect(found.LastTransitionTime.Time).To(BeTemporally(">", beforeUpdate.LastTransitionTime.Time))

	})
})
