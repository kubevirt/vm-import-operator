package conditions

import (
	"time"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCondition creates condition of a given type in the conditions list with given reason, message and status
func NewCondition(conditionType v2vv1alpha1.VirtualMachineImportConditionType, reason string, message string, status v1.ConditionStatus) v2vv1alpha1.VirtualMachineImportCondition {
	now := metav1.NewTime(time.Now())
	condition := v2vv1alpha1.VirtualMachineImportCondition{
		Type:               conditionType,
		LastTransitionTime: &now,
		LastHeartbeatTime:  &now,
		Message:            &message,
		Reason:             &reason,
		Status:             status,
	}
	return condition
}

// UpsertCondition updates or creates condition in the virtualMachineImportStatus
func UpsertCondition(vmi *v2vv1alpha1.VirtualMachineImport, condition v2vv1alpha1.VirtualMachineImportCondition) {
	existingCondition := FindConditionOfType(vmi.Status.Conditions, condition.Type)
	now := metav1.NewTime(time.Now())

	if existingCondition != nil {
		existingCondition.Message = condition.Message
		existingCondition.Reason = condition.Reason
		existingCondition.LastHeartbeatTime = &now
		if existingCondition.Status != condition.Status {
			existingCondition.Status = condition.Status
			existingCondition.LastTransitionTime = condition.LastTransitionTime
		}
	} else {
		vmi.Status.Conditions = append(vmi.Status.Conditions, condition)
	}
}

// FindConditionOfType finds condition of a conditionType type in the conditions slice
func FindConditionOfType(conditions []v2vv1alpha1.VirtualMachineImportCondition, conditionType v2vv1alpha1.VirtualMachineImportConditionType) *v2vv1alpha1.VirtualMachineImportCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
