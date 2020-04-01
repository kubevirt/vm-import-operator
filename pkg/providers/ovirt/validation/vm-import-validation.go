package validation

import (
	"fmt"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/utils"

	"github.com/kubevirt/vm-import-operator/pkg/conditions"
	validators "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"

	ovirtsdk "github.com/ovirt/go-ovirt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type action int

var logger = logf.Log.WithName("validation")

const (
	log   = 0
	warn  = 1
	block = 2

	warnReason  = string(v2vv1alpha1.MappingRulesCheckingReportedWarnings)
	errorReason = string(v2vv1alpha1.MappingRulesCheckingFailed)
	okReason    = string(v2vv1alpha1.MappingRulesCheckingCompleted)

	incompleteMappingRulesReason = string(v2vv1alpha1.IncompleteMappingRules)
	validationCompletedReason    = string(v2vv1alpha1.ValidationCompleted)
)

var checkToAction = map[validators.CheckID]action{
	// NIC rules
	validators.NicInterfaceCheckID:       block,
	validators.NicOnBootID:               log,
	validators.NicPluggedID:              warn,
	validators.NicVNicPassThroughID:      block,
	validators.NicVNicPortMirroringID:    warn,
	validators.NicVNicCustomPropertiesID: warn,
	validators.NicVNicNetworkFilterID:    warn,
	validators.NicVNicQosID:              log,
	// Storage rules
	validators.DiskAttachmentInterfaceID:           block,
	validators.DiskAttachmentLogicalNameID:         log,
	validators.DiskAttachmentPassDiscardID:         log,
	validators.DiskAttachmentUsesScsiReservationID: block,
	validators.DiskInterfaceID:                     block,
	validators.DiskLogicalNameID:                   log,
	validators.DiskUsesScsiReservationID:           block,
	validators.DiskBackupID:                        warn,
	validators.DiskLunStorageID:                    block,
	validators.DiskPropagateErrorsID:               log,
	validators.DiskWipeAfterDeleteID:               log,
	validators.DiskStatusID:                        block,
	validators.DiskStoragaTypeID:                   block,
	validators.DiskSgioID:                          block,
	// VM rules
	validators.VMBiosBootMenuID:                  log,
	validators.VMBiosTypeID:                      block,
	validators.VMBiosTypeQ35SecureBootID:         warn,
	validators.VMCpuArchitectureID:               block,
	validators.VMCpuTuneID:                       warn,
	validators.VMCpuSharesID:                     log,
	validators.VMCustomPropertiesID:              warn,
	validators.VMDisplayTypeID:                   log,
	validators.VMHasIllegalImagesID:              block,
	validators.VMHighAvailabilityPriorityID:      log,
	validators.VMIoThreadsID:                     warn,
	validators.VMMemoryPolicyBallooningID:        log,
	validators.VMMemoryPolicyOvercommitPercentID: log,
	validators.VMMemoryPolicyGuaranteedID:        log,
	validators.VMMigrationID:                     log,
	validators.VMMigrationDowntimeID:             log,
	validators.VMNumaTuneModeID:                  warn,
	validators.VMOriginID:                        block,
	validators.VMRngDeviceSourceID:               log,
	validators.VMSoundcardEnabledID:              warn,
	validators.VMStartPausedID:                   log,
	validators.VMStorageErrorResumeBehaviourID:   log,
	validators.VMTunnelMigrationID:               warn,
	validators.VMUsbID:                           block,
	validators.VMGraphicConsolesID:               log,
	validators.VMHostDevicesID:                   log,
	validators.VMReportedDevicesID:               log,
	validators.VMQuotaID:                         log,
	validators.VMWatchdogsID:                     block,
	validators.VMCdromsID:                        log,
	validators.VMFloppiesID:                      log,
}

// Validator validates different properties of a VM
type Validator interface {
	ValidateVM(vm *ovirtsdk.Vm) []validators.ValidationFailure
	ValidateDiskAttachments(diskAttachments []*ovirtsdk.DiskAttachment) []validators.ValidationFailure
	ValidateNics(nics []*ovirtsdk.Nic) []validators.ValidationFailure
	ValidateNetworkMapping(nics []*ovirtsdk.Nic, mapping *[]v2vv1alpha1.ResourceMappingItem, crNamespace string) []validators.ValidationFailure
	ValidateStorageMapping(attachments []*ovirtsdk.DiskAttachment, mapping *[]v2vv1alpha1.ResourceMappingItem) []validators.ValidationFailure
}

// VirtualMachineImportValidator validates VirtualMachineImport object
type VirtualMachineImportValidator struct {
	Validator Validator
}

// NewVirtualMachineImportValidator creates ready-to-use NewVirtualMachineImportValidator
func NewVirtualMachineImportValidator(validator Validator) VirtualMachineImportValidator {
	return VirtualMachineImportValidator{
		Validator: validator,
	}
}

// Validate validates whether VM described in VirtualMachineImport can be imported
func (validator *VirtualMachineImportValidator) Validate(vm *ovirtsdk.Vm, vmiCrName *types.NamespacedName, mappings *v2vv1alpha1.OvirtMappings) []v2vv1alpha1.VirtualMachineImportCondition {
	var validationResults []v2vv1alpha1.VirtualMachineImportCondition
	mappingsCheckResult := validator.validateMappings(vm, mappings, vmiCrName)
	validationResults = append(validationResults, mappingsCheckResult)

	failures := validator.Validator.ValidateVM(vm)
	if nics, ok := vm.Nics(); ok {

		failures = append(failures, validator.Validator.ValidateNics(nics.Slice())...)
	}
	if das, ok := vm.DiskAttachments(); ok {
		failures = append(failures, validator.Validator.ValidateDiskAttachments(das.Slice())...)
	}
	rulesCheckResult := validator.processValidationFailures(failures, vmiCrName)
	validationResults = append(validationResults, rulesCheckResult)
	return validationResults
}

func (validator *VirtualMachineImportValidator) validateMappings(vm *ovirtsdk.Vm, mappings *v2vv1alpha1.OvirtMappings, vmiCrName *types.NamespacedName) v2vv1alpha1.VirtualMachineImportCondition {
	var failures []validators.ValidationFailure

	if nics, ok := vm.Nics(); ok {
		nSlice := nics.Slice()
		failures = append(failures, validator.Validator.ValidateNetworkMapping(nSlice, mappings.NetworkMappings, vmiCrName.Namespace)...)
	}
	if attachments, ok := vm.DiskAttachments(); ok {
		das := attachments.Slice()
		failures = append(failures, validator.Validator.ValidateStorageMapping(das, mappings.StorageMappings)...)
	}

	return validator.processMappingValidationFailures(failures, vmiCrName)
}

func (validator *VirtualMachineImportValidator) processMappingValidationFailures(failures []validators.ValidationFailure, vmiCrName *types.NamespacedName) v2vv1alpha1.VirtualMachineImportCondition {
	var message string

	for _, failure := range failures {
		message = utils.WithMessage(message, failure.Message)
	}
	if len(failures) > 0 {
		return conditions.NewCondition(v2vv1alpha1.Validating, incompleteMappingRulesReason, message, v1.ConditionFalse)
	}
	return conditions.NewCondition(v2vv1alpha1.Validating, validationCompletedReason, "Validating completed successfully", v1.ConditionTrue)
}

func (validator *VirtualMachineImportValidator) processValidationFailures(failures []validators.ValidationFailure, vmiCrName *types.NamespacedName) v2vv1alpha1.VirtualMachineImportCondition {
	valid := true
	var warnMessage, errorMessage string

	for _, failure := range failures {
		switch checkToAction[failure.ID] {
		case log:
			logger.Info(fmt.Sprintf("Validation information for %v: %v", vmiCrName, failure))
		case warn:
			warnMessage = utils.WithMessage(warnMessage, failure.Message)
		case block:
			valid = false
			errorMessage = utils.WithMessage(errorMessage, failure.Message)
		}
	}

	if !valid {
		return conditions.NewCondition(v2vv1alpha1.MappingRulesChecking, errorReason, errorMessage, v1.ConditionFalse)
	} else if warnMessage != "" {
		return conditions.NewCondition(v2vv1alpha1.MappingRulesChecking, warnReason, warnMessage, v1.ConditionTrue)
	} else {
		return conditions.NewCondition(v2vv1alpha1.MappingRulesChecking, okReason, "All mapping rules checks passed", v1.ConditionTrue)
	}
}
