package validation

import (
	"encoding/json"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	validators "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Validating VirtualMachineImport Admitter", func() {
	var vmImportValidator *VirtualMachineImportValidator
	var clientMock mockClient

	BeforeEach(func() {
		statusUpdateObjects = nil
		clientMock = mockClient{
			statusWriter: mockStatusWriter{},
		}
		vmImportValidator = &VirtualMachineImportValidator{
			Validator: &mockValidator{},
			client:    &clientMock,
		}
		validateVMMock = func(vm *ovirtsdk.Vm) []validators.ValidationFailure {
			return []validators.ValidationFailure{}
		}
		validateNicsMock = func(nics []*ovirtsdk.Nic) []validators.ValidationFailure {
			return []validators.ValidationFailure{}
		}
		validateDiskAttachmentsMock = func(attachments []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
			return []validators.ValidationFailure{}
		}
		clientGetMock = func(obj runtime.Object) error {
			cr := v2vv1alpha1.VirtualMachineImport{}
			data, _ := json.Marshal(cr)
			decoderFor(cr.GroupVersionKind().GroupVersion()).Decode(data, nil, obj)
			return nil
		}
	})
	It("should accept VirtualMachineImport", func() {
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Reason).To(Equal(okReason))
	})

	It("should update existing VirtualMachineImport status condition ", func() {
		vm := newVM()
		crName := newNamespacedName()
		reason := errorReason
		message := "Error message"
		clientGetMock = func(obj runtime.Object) error {
			cr := v2vv1alpha1.VirtualMachineImport{
				Status: v2vv1alpha1.VirtualMachineImportStatus{
					Conditions: []v2vv1alpha1.VirtualMachineImportCondition{
						v2vv1alpha1.VirtualMachineImportCondition{
							Type:    v2vv1alpha1.Processing,
							Reason:  &reason,
							Message: &message,
							Status:  v1.ConditionFalse,
						},
					},
				},
			}
			data, _ := json.Marshal(cr)
			decoderFor(cr.GroupVersionKind().GroupVersion()).Decode(data, nil, obj)
			return nil
		}
		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Reason).To(Equal(okReason))
	})
	table.DescribeTable("should accept VirtualMachineImport spec with VM log for ", func(checkId validators.CheckID) {
		message := "Some log"

		validateVMMock = func(_ *ovirtsdk.Vm) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Reason).To(Equal(okReason))
	},
		table.Entry("Boot menu", validators.VMBiosBootMenuID),
		table.Entry("CPU shares", validators.VMCpuSharesID),
		table.Entry("Display type", validators.VMDisplayTypeID),
		table.Entry("HA priority", validators.VMHighAvailabilityPriorityID),
		table.Entry("Ballooning", validators.VMMemoryPolicyBallooningID),
		table.Entry("Overcommit %", validators.VMMemoryPolicyOvercommitPercentID),
		table.Entry("Guaranteed memory", validators.VMMemoryPolicyGuaranteedID),
		table.Entry("Migration", validators.VMMigrationID),
		table.Entry("Migration downtime", validators.VMMigrationDowntimeID),
		table.Entry("RNG device", validators.VMRngDeviceSourceID),
		table.Entry("Start paused", validators.VMStartPausedID),
		table.Entry("Storate error resume behaviour", validators.VMStorageErrorResumeBehaviourID),
		table.Entry("Graphic consoles", validators.VMGraphicConsolesID),
		table.Entry("Host devices", validators.VMHostDevicesID),
		table.Entry("Reported devices", validators.VMReportedDevicesID),
		table.Entry("Quote", validators.VMQuotaID),
		table.Entry("CD roms", validators.VMCdromsID),
		table.Entry("Floppies", validators.VMFloppiesID),
	)
	table.DescribeTable("should accept VirtualMachineImport spec with VM warning for ", func(checkId validators.CheckID) {
		message := "Some warning"

		validateVMMock = func(_ *ovirtsdk.Vm) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Message).To(ContainSubstring(message))
		Expect(*condition.Reason).To(Equal(warnReason))

	},
		table.Entry("Q35 Secure boot", validators.VMBiosTypeQ35SecureBootID),
		table.Entry("CPU Tune", validators.VMCpuTuneID),
		table.Entry("Custom properties", validators.VMCustomPropertiesID),
		table.Entry("I/O Threads", validators.VMIoThreadsID),
		table.Entry("NUMA tune mode", validators.VMNumaTuneModeID),
		table.Entry("Sound card", validators.VMSoundcardEnabledID),
		table.Entry("Tunnel migration", validators.VMTunnelMigrationID),
	)
	table.DescribeTable("should reject VirtualMachineImport spec for ", func(checkId validators.CheckID) {
		message := "Blocked!"

		validateVMMock = func(_ *ovirtsdk.Vm) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(Not(BeNil()))
		Expect(err.Error()).To(ContainSubstring(message))
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionFalse))
		Expect(*condition.Message).To(ContainSubstring(message))
		Expect(*condition.Reason).To(Equal(errorReason))
	},
		table.Entry("Bios type", validators.VMBiosTypeID),
		table.Entry("CPU Architecture", validators.VMCpuArchitectureID),
		table.Entry("Illegal images", validators.VMHasIllegalImagesID),
		table.Entry("Origin ID", validators.VMOriginID),
		table.Entry("USB", validators.VMUsbID),
		table.Entry("Watchdog", validators.VMWatchdogsID),
	)
	table.DescribeTable("should accept VirtualMachineImport spec with Nic log for ", func(checkId validators.CheckID) {
		message := "Some log"

		validateNicsMock = func(_ []*ovirtsdk.Nic) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Reason).To(BeEquivalentTo(okReason))

	},
		table.Entry("On Boot", validators.NicOnBootID),
		table.Entry("QOS", validators.NicVNicQosID),
	)
	table.DescribeTable("should accept VirtualMachineImport spec with Nic warning for ", func(checkId validators.CheckID) {
		message := "Some warning"

		validateNicsMock = func(_ []*ovirtsdk.Nic) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Message).To(ContainSubstring(message))
		Expect(*condition.Reason).To(Equal(warnReason))
	},
		table.Entry("Plugged", validators.NicPluggedID),
		table.Entry("Port mirroring", validators.NicVNicPortMirroringID),
		table.Entry("Custom properties", validators.NicVNicCustomPropertiesID),
		table.Entry("Network filter", validators.NicVNicNetworkFilterID),
	)
	table.DescribeTable("should reject VirtualMachineImport spec for Nic ", func(checkId validators.CheckID) {
		message := "Blocked!"

		validateNicsMock = func(_ []*ovirtsdk.Nic) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(Not(BeNil()))
		Expect(err.Error()).To(ContainSubstring(message))
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionFalse))
		Expect(*condition.Message).To(ContainSubstring(message))
		Expect(*condition.Reason).To(Equal(errorReason))
	},
		table.Entry("Interface model", validators.NicInterfaceCheckID),
		table.Entry("Pass-through", validators.NicVNicPassThroughID),
	)
	table.DescribeTable("should accept VirtualMachineImport spec with storage log for ", func(checkId validators.CheckID) {
		message := "Some log"

		validateDiskAttachmentsMock = func(_ []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Reason).To(Equal(okReason))

	},
		table.Entry("Disk attachment logical name", validators.DiskAttachmentLogicalNameID),
		table.Entry("Disk logical name", validators.DiskLogicalNameID),
		table.Entry("Disk attachment pass discard", validators.DiskAttachmentPassDiscardID),
		table.Entry("Disk propagate errors", validators.DiskPropagateErrorsID),
		table.Entry("Disk wipe after delete", validators.DiskWipeAfterDeleteID),
	)
	table.DescribeTable("should accept VirtualMachineImport spec with storage warning for ", func(checkId validators.CheckID) {
		message := "Some warning"

		validateDiskAttachmentsMock = func(_ []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(BeNil())
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionTrue))
		Expect(*condition.Message).To(ContainSubstring(message))
		Expect(*condition.Reason).To(Equal(warnReason))
	},
		table.Entry("Disk backup", validators.DiskBackupID),
	)
	table.DescribeTable("should reject VirtualMachineImport spec for storage ", func(checkId validators.CheckID) {
		message := "Blocked!"

		validateDiskAttachmentsMock = func(_ []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
			return oneValidationFailure(checkId, message)
		}
		vm := newVM()
		crName := newNamespacedName()

		err := vmImportValidator.Validate(vm, crName)

		Expect(err).To(Not(BeNil()))
		Expect(err.Error()).To(ContainSubstring(message))
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionFalse))
		Expect(*condition.Message).To(ContainSubstring(message))
		Expect(*condition.Reason).To(Equal(errorReason))
	},
		table.Entry("Disk attachment interface model", validators.DiskAttachmentInterfaceID),
		table.Entry("Disk interface model", validators.DiskInterfaceID),
		table.Entry("Disk attachment uses scsi reservation", validators.DiskAttachmentUsesScsiReservationID),
		table.Entry("Disk uses scsi reservation", validators.DiskUsesScsiReservationID),
		table.Entry("Disk uses LUN", validators.DiskLunStorageID),
		table.Entry("Disk status", validators.DiskStatusID),
		table.Entry("Disk SGIO", validators.DiskSgioID),
	)
	It("should reject VirtualMachineImport spec with vm, nic and storage blocks ", func() {
		vm := newVM()
		crName := newNamespacedName()

		storageFailure1 := validators.ValidationFailure{
			ID:      validators.DiskLunStorageID,
			Message: "Lun storage",
		}
		storageFailure2 := validators.ValidationFailure{
			ID:      validators.DiskStatusID,
			Message: "Status",
		}
		validateDiskAttachmentsMock = func(_ []*ovirtsdk.DiskAttachment) []validators.ValidationFailure {
			return []validators.ValidationFailure{
				storageFailure1, storageFailure2,
			}
		}

		vmFailure1 := validators.ValidationFailure{
			ID:      validators.VMBiosTypeID,
			Message: "BIOS type",
		}
		vmFailure2 := validators.ValidationFailure{
			ID:      validators.VMCpuArchitectureID,
			Message: "CPU Architecture",
		}
		validateVMMock = func(_ *ovirtsdk.Vm) []validators.ValidationFailure {
			return []validators.ValidationFailure{
				vmFailure1, vmFailure2,
			}
		}

		nicFailure1 := validators.ValidationFailure{
			ID:      validators.NicInterfaceCheckID,
			Message: "Interface model",
		}
		nicFailure2 := validators.ValidationFailure{
			ID:      validators.NicVNicPassThroughID,
			Message: "Pass-through",
		}
		validateNicsMock = func(_ []*ovirtsdk.Nic) []validators.ValidationFailure {
			return []validators.ValidationFailure{
				nicFailure1, nicFailure2,
			}
		}

		err := vmImportValidator.Validate(vm, crName)
		Expect(err).To(Not(BeNil()))
		Expect(err.Error()).To(ContainSubstring(storageFailure1.Message))
		Expect(err.Error()).To(ContainSubstring(storageFailure2.Message))
		Expect(err.Error()).To(ContainSubstring(nicFailure1.Message))
		Expect(err.Error()).To(ContainSubstring(nicFailure2.Message))
		Expect(err.Error()).To(ContainSubstring(vmFailure1.Message))
		Expect(err.Error()).To(ContainSubstring(vmFailure2.Message))
		Expect(statusUpdateObjects).To(HaveLen(1))
		vmi := statusUpdateObjects[0].(*v2vv1alpha1.VirtualMachineImport)
		Expect(vmi.Status.Conditions).To(HaveLen(1))
		condition := vmi.Status.Conditions[0]
		Expect(condition.Type).To(Equal(v2vv1alpha1.Processing))
		Expect(condition.Status).To(Equal(v1.ConditionFalse))
		Expect(*condition.Message).To(ContainSubstring(storageFailure1.Message))
		Expect(*condition.Message).To(ContainSubstring(storageFailure2.Message))
		Expect(*condition.Message).To(ContainSubstring(nicFailure1.Message))
		Expect(*condition.Message).To(ContainSubstring(nicFailure2.Message))
		Expect(*condition.Message).To(ContainSubstring(vmFailure1.Message))
		Expect(*condition.Message).To(ContainSubstring(vmFailure2.Message))
		Expect(*condition.Reason).To(Equal(errorReason))
	})
})

func oneValidationFailure(checkID validators.CheckID, message string) []validators.ValidationFailure {
	return []validators.ValidationFailure{
		validators.ValidationFailure{
			ID:      checkID,
			Message: message,
		},
	}
}

func newVM() *ovirtsdk.Vm {
	vm := ovirtsdk.Vm{}
	nicSlice := ovirtsdk.NicSlice{}
	nicSlice.SetSlice([]*ovirtsdk.Nic{&ovirtsdk.Nic{}})
	vm.SetNics(&nicSlice)

	daSlice := ovirtsdk.DiskAttachmentSlice{}
	daSlice.SetSlice([]*ovirtsdk.DiskAttachment{&ovirtsdk.DiskAttachment{}})
	vm.SetDiskAttachments(&daSlice)

	return &vm
}

func newNamespacedName() *types.NamespacedName {
	nn := types.NamespacedName{
		Name:      "foo",
		Namespace: "bar",
	}
	return &nn
}

func decoderFor(gv schema.GroupVersion) runtime.Decoder {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(gv)
	codecs := serializer.NewCodecFactory(scheme)
	return codecs.UniversalDecoder()
}
