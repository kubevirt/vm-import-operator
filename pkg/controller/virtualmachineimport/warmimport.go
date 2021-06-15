package virtualmachineimport

import (
	"context"
	"errors"
	"fmt"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	"time"

	"github.com/go-logr/logr"
	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// if the finalization date is already past and we haven't captured a snapshot yet,
// then we should just proceed with a cold import.
func skipWarmImport(instance *v2vv1.VirtualMachineImport) bool {
	return shouldFinalizeWarmImport(instance) && instance.Status.WarmImport.RootSnapshot == nil
}

func shouldWarmImport(provider provider.Provider, instance *v2vv1.VirtualMachineImport) bool {
	return provider.SupportsWarmMigration() && instance.Spec.Warm && !skipWarmImport(instance)
}

func shouldFinalizeWarmImport(instance *v2vv1.VirtualMachineImport) bool {
	return instance.Spec.Warm && instance.Spec.FinalizeDate != nil && !instance.Spec.FinalizeDate.After(time.Now())
}

func (r *ReconcileVirtualMachineImport) warmImport(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName, log logr.Logger) (time.Duration, error) {
	if instance.Status.WarmImport.Failures > r.ctrlConfig.WarmImportMaxFailures() || instance.Status.WarmImport.ConsecutiveFailures > r.ctrlConfig.WarmImportConsecutiveFailures() {
		err := r.endWarmImportFailed(provider, instance, "warm import retry limit reached")
		if err != nil {
			return NoReQ, err
		}
	}

	err := utils.AddFinalizer(instance, utils.CleanupSnapshotsFinalizer, r.client)
	if err != nil {
		return FastReQ, err
	}

	created, err := r.ensureDisksExist(provider, instance, mapper, vmName)
	if err != nil {
		return FastReQ, err
	}

	// if the disks don't exist yet, requeue til they do
	if !created {
		log.Info("Waiting for data volumes to be created")
		return FastReQ, nil
	}

	// if the datavolumes are waiting for first consumer, then
	// attempt to start the VM so that Kubevirt can schedule
	// it and allow the datavolumes to bind.
	waiting, err := r.dvsWaitingForFirstConsumer(instance, mapper, vmName)
	if err != nil {
		return FastReQ, err
	}
	if waiting {
		log.Info("Waiting for data volumes to be bound.")
		err = r.setRunning(vmName, true)
		if err != nil {
			return FastReQ, err
		}
	} else {
		// restore the original running state if the datavolumes are no longer waiting.
		err = r.setRunning(vmName, mapper.RunningState())
		if err != nil {
			return FastReQ, err
		}
	}

	// if the stage isn't complete yet (all dvs paused), requeue till it's done
	complete, err := r.isStageComplete(instance, mapper, vmName)
	if err != nil {
		return SlowReQ, err
	}
	if !complete {
		err := r.setWarmImportCondition(instance, v2vv1.CopyingStage, fmt.Sprintf("Copying next warm import stage"))
		if err != nil {
			return FastReQ, err
		}

		log.Info("Waiting for warm import iteration to complete")
		return SlowReQ, nil
	}

	// should only run after the very first stage is completed.
	if instance.Status.WarmImport.NextStageTime == nil {
		err = r.setNextStageTime(instance)
		if err != nil {
			return FastReQ, err
		}
	}

	now := metav1.Now()
	// if a warm import has been started but it's not time for the next stage, requeue the event
	if instance.Status.WarmImport.NextStageTime != nil && instance.Status.WarmImport.NextStageTime.After(now.Time) {
		log.Info("Waiting for next warm import stage")
		err := r.setWarmImportCondition(instance, v2vv1.CopyingPaused, fmt.Sprintf("Waiting for next warm import stage"))
		return SlowReQ, err
	}

	err = r.setupNextStage(provider, instance, mapper, vmName, false)
	if err != nil {
		return FastReQ, err
	}

	err = r.setNextStageTime(instance)
	if err != nil {
		return FastReQ, err
	}

	// always requeue, since shouldWarmImport will return false when it's time to finalize.
	log.Info("Commencing next warm import stage")
	return FastReQ, nil
}

func (r *ReconcileVirtualMachineImport) ensureDisksExist(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName) (bool, error) {
	dvs, err := mapper.MapDataVolumes(&vmName.Name, r.filesystemOverhead)
	if err != nil {
		return false, err
	}

	for dvID, dvDef := range dvs {
		dvName := types.NamespacedName{Namespace: instance.Namespace, Name: dvID}

		dv, err := r.getDataVolume(dvName)
		if err != nil {
			return false, err
		}
		if dv != nil {
			continue
		}

		// We have to validate the disk status, so we are sure, the disk wasn't manipulated,
		// before we execute the import:
		valid, err := provider.ValidateDiskStatus(dvName.Name)
		if err != nil {
			return false, err
		}
		if !valid {
			err := r.endDiskImportFailed(provider, instance, dv, "disk is in illegal status")
			if err != nil {
				return false, err
			}
		}

		// only take a snapshot if one hasn't been taken yet
		var snapshotRef string
		if instance.Status.WarmImport.RootSnapshot != nil {
			snapshotRef = *instance.Status.WarmImport.RootSnapshot
		} else {
			snapshotRef, err = provider.CreateVMSnapshot()
			if err != nil {
				_ = r.incrementWarmImportFailures(instance)
				return false, err
			}
			err = r.setRootSnapshot(instance, snapshotRef)
			if err != nil {
				return false, err
			}
		}

		dvDef.Spec.FinalCheckpoint = false
		dvDef.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
			{Previous: "", Current: snapshotRef},
		}
		dv, err = r.createDataVolume(provider, mapper, instance, &dvDef, vmName)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (r *ReconcileVirtualMachineImport) isStageComplete(instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName) (bool, error) {
	disksDoneStage := 0
	dvs, err := mapper.MapDataVolumes(&vmName.Name, r.filesystemOverhead)
	if err != nil {
		return false, err
	}
	for dvID, _ := range dvs {
		dvName := types.NamespacedName{Namespace: instance.Namespace, Name: dvID}
		dv, err := r.getDataVolume(dvName)
		if err != nil {
			return false, err
		}
		if dv == nil {
			return false, nil
		}

		if dv.Status.Phase == cdiv1.Failed {
			_ = r.incrementWarmImportFailures(instance)
			return false, errors.New("DataVolume stage failed")
		}

		if dv.Status.Phase == cdiv1.Paused || dv.Status.Phase == cdiv1.Succeeded {
			disksDoneStage++
		}
	}

	return disksDoneStage == len(dvs), nil
}

func (r *ReconcileVirtualMachineImport) setupNextStage(provider provider.Provider, instance *v2vv1.VirtualMachineImport, mapper provider.Mapper, vmName types.NamespacedName, final bool) error {
	var snapshotRef string

	dvs, err := mapper.MapDataVolumes(&vmName.Name, r.filesystemOverhead)
	if err != nil {
		return err
	}

	for dvID, _ := range dvs {
		dvName := types.NamespacedName{Namespace: instance.Namespace, Name: dvID}
		dv := &cdiv1.DataVolume{}
		err := r.client.Get(context.TODO(), dvName, dv)
		if err != nil {
			return err
		}
		if dv.Spec.FinalCheckpoint {
			return nil
		}

		if snapshotRef == "" {
			snapshotRef, err = provider.CreateVMSnapshot()
			if err != nil {
				_ = r.incrementWarmImportFailures(instance)
				return err
			}
		}

		// this shouldn't happen, since the initial checkpoint should
		// have been set when the DV was created.
		if dv.Spec.Checkpoints == nil || len(dv.Spec.Checkpoints) == 0 {
			dv.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{Previous: "", Current: snapshotRef},
			}
		} else {
			numCheckpoints := len(dv.Spec.Checkpoints)
			lastCheckpoint := dv.Spec.Checkpoints[numCheckpoints-1]
			newCheckpoint := cdiv1.DataVolumeCheckpoint{
				Previous: lastCheckpoint.Current,
				Current:  snapshotRef,
			}
			dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, newCheckpoint)
			dv.Spec.FinalCheckpoint = final
		}
		err = r.client.Update(context.TODO(), dv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileVirtualMachineImport) setRootSnapshot(instance *v2vv1.VirtualMachineImport, snapshotRef string) error {
	instanceCopy := instance.DeepCopy()
	instance.Status.WarmImport.RootSnapshot = &snapshotRef

	patch := client.MergeFrom(instanceCopy)
	err := r.client.Status().Patch(context.TODO(), instance, patch)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileVirtualMachineImport) setNextStageTime(instance *v2vv1.VirtualMachineImport) error {
	// we already have the next stage scheduled
	if instance.Status.WarmImport.NextStageTime != nil && instance.Status.WarmImport.NextStageTime.After(time.Now()) {
		return nil
	}

	instanceCopy := instance.DeepCopy()
	patch := client.MergeFrom(instanceCopy)
	nextStageTime := metav1.NewTime(time.Now().Add(time.Duration(r.ctrlConfig.WarmImportIntervalMinutes()) * time.Minute))
	instance.Status.WarmImport.Successes += 1
	instance.Status.WarmImport.ConsecutiveFailures = 0
	instance.Status.WarmImport.NextStageTime = &nextStageTime

	return r.client.Status().Patch(context.TODO(), instance, patch)
}

func (r *ReconcileVirtualMachineImport) setFinalCheckpoint(dv *cdiv1.DataVolume) error {
	// already set
	if dv.Spec.FinalCheckpoint {
		return nil
	}
	dvCopy := dv.DeepCopy()
	dvCopy.Spec.FinalCheckpoint = true

	patch := client.MergeFrom(dv)
	return r.client.Patch(context.TODO(), dvCopy, patch)
}

func (r *ReconcileVirtualMachineImport) setWarmImportCondition(instance *v2vv1.VirtualMachineImport, reason v2vv1.ProcessingConditionReason, message string) error {
	processingCond := conditions.NewProcessingCondition(string(reason), message, corev1.ConditionTrue)
	return r.upsertStatusConditions(types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, processingCond)
}

func (r *ReconcileVirtualMachineImport) incrementWarmImportFailures(instance *v2vv1.VirtualMachineImport) error {
	instance.Status.WarmImport.Failures += 1
	instance.Status.WarmImport.ConsecutiveFailures += 1
	return r.client.Status().Update(context.TODO(), instance)
}
