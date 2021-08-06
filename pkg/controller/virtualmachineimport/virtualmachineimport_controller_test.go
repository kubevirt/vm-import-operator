package virtualmachineimport

import (
	"context"
	"fmt"

	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"
	"github.com/kubevirt/vm-import-operator/pkg/metrics"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	pclient "github.com/kubevirt/vm-import-operator/pkg/client"
	"github.com/kubevirt/vm-import-operator/pkg/mappings"
	"github.com/kubevirt/vm-import-operator/pkg/ownerreferences"
	provider "github.com/kubevirt/vm-import-operator/pkg/providers"
	oapiv1 "github.com/openshift/api/template/v1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var (
	get                      func(context.Context, client.ObjectKey, runtime.Object) error
	pinit                    func(*corev1.Secret, *v2vv1.VirtualMachineImport) error
	loadVM                   func(v2vv1.VirtualMachineImportSourceSpec) error
	getResourceMapping       func(types.NamespacedName) (*v2vv1.ResourceMapping, error)
	validate                 func() ([]v2vv1.VirtualMachineImportCondition, error)
	statusPatch              func(ctx context.Context, obj runtime.Object, patch client.Patch) error
	getVMStatus              func() (provider.VMStatus, error)
	findTemplate             func() (*oapiv1.Template, error)
	processTemplate          func(template *oapiv1.Template, name *string, namespace string) (*kubevirtv1.VirtualMachine, error)
	create                   func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error
	cleanUp                  func() error
	update                   func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error
	mapDisks                 func() (map[string]cdiv1.DataVolume, error)
	getVM                    func(id *string, name *string, cluster *string, clusterID *string) (interface{}, error)
	stopVM                   func(id string) error
	list                     func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error
	getCtrlConfig            func() ctrlConfig.ControllerConfig
	needsGuestConversion     func() bool
	getGuestConversionPod    func() (*corev1.Pod, error)
	launchGuestConversionPod func() (*corev1.Pod, error)
	supportsWarmMigration    func() bool
	createVMSnapshot         func() (string, error)
	removeVMSnapshot         func(string, bool) error
)

var _ = Describe("Reconcile steps", func() {
	var (
		reconciler *ReconcileVirtualMachineImport
		instance   *v2vv1.VirtualMachineImport
		vmName     types.NamespacedName
		mock       *mockProvider
	)

	BeforeEach(func() {
		instance = &v2vv1.VirtualMachineImport{}
		mockClient := &mockClient{}
		finder := &mockFinder{}
		scheme := runtime.NewScheme()
		factory := &mockFactory{}
		controller := &mockController{}
		ctrlConfigProviderMock := &mockControllerConfigProvider{}
		scheme.AddKnownTypes(v2vv1.SchemeGroupVersion,
			&v2vv1.VirtualMachineImport{},
		)
		scheme.AddKnownTypes(cdiv1.SchemeGroupVersion,
			&cdiv1.DataVolume{},
		)
		scheme.AddKnownTypes(kubevirtv1.GroupVersion,
			&kubevirtv1.VirtualMachine{},
		)

		get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
			switch obj.(type) {
			case *v2vv1.VirtualMachineImport:
				obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{}
			case *v2vv1.ResourceMapping:
				obj.(*v2vv1.ResourceMapping).Spec = v2vv1.ResourceMappingSpec{}
			case *corev1.Secret:
				obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
			}
			return nil
		}
		statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
			return nil
		}
		validate = func() ([]v2vv1.VirtualMachineImportCondition, error) {
			return []v2vv1.VirtualMachineImportCondition{}, nil
		}
		getVMStatus = func() (provider.VMStatus, error) {
			return provider.VMStatusDown, nil
		}
		create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
			return nil
		}
		cleanUp = func() error {
			return nil
		}
		getCtrlConfig = func() ctrlConfig.ControllerConfig {
			return ctrlConfig.ControllerConfig{}
		}
		update = func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
			return nil
		}
		needsGuestConversion = func() bool {
			return false
		}
		vmName = types.NamespacedName{Name: "test", Namespace: "default"}
		rec := record.NewFakeRecorder(2)

		reconciler = NewReconciler(mockClient, finder, scheme, ownerreferences.NewOwnerReferenceManager(mockClient), factory, rec, controller, ctrlConfigProviderMock)
	})

	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	Describe("Init steps", func() {
		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1.VirtualMachineImportOvirtSourceSpec{}
			instance.Name = "test"
			instance.Namespace = "test"
		})

		It("should fail to create provider: ", func() {
			instance.Spec.Source.Ovirt = nil

			provider, err := reconciler.createProvider(instance)

			Expect(provider).To(BeNil())
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(Equal("Invalid source type. Only Ovirt and Vmware type is supported"))
		})

		It("should fail to create provider if more than one source is provided: ", func() {
			instance.Spec.Source.Ovirt = &v2vv1.VirtualMachineImportOvirtSourceSpec{}
			instance.Spec.Source.Vmware = &v2vv1.VirtualMachineImportVmwareSourceSpec{}

			provider, err := reconciler.createProvider(instance)

			Expect(provider).To(BeNil())
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(Equal("Invalid source. Must only include one source type."))
		})

		It("should create provider: ", func() {
			provider, err := reconciler.createProvider(instance)

			Expect(provider).To(Not(BeNil()))
			Expect(err).To(BeNil())
		})
	})

	Describe("Create steps", func() {

		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1.VirtualMachineImportOvirtSourceSpec{}
			instance.Name = "test"
			instance.Namespace = "test"

			mock = &mockProvider{}
		})

		It("should fail to initate provider with missing secret: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Not there")
			}

			msg, err := reconciler.initProvider(instance, mock)

			Expect(msg).To(Equal("Failed to read the secret"))
			Expect(err).To(Not(BeNil()))
		})
	})

	Describe("fetchVM step", func() {

		BeforeEach(func() {
			pinit = func(*corev1.Secret, *v2vv1.VirtualMachineImport) error {
				return nil
			}
			loadVM = func(v2vv1.VirtualMachineImportSourceSpec) error {
				return nil
			}
			getResourceMapping = func(types.NamespacedName) (*v2vv1.ResourceMapping, error) {
				return &v2vv1.ResourceMapping{
					Spec: v2vv1.ResourceMappingSpec{},
				}, nil
			}
			instance.Spec.Source.Ovirt = &v2vv1.VirtualMachineImportOvirtSourceSpec{}
			instance.Name = "test"
			instance.Namespace = "test"

			mock = &mockProvider{}
		})

		It("should fail to connect: ", func() {
			pinit = func(*corev1.Secret, *v2vv1.VirtualMachineImport) error {
				return fmt.Errorf("Not connected")
			}

			msg, err := reconciler.initProvider(instance, mock)

			Expect(msg).To(Equal("Source provider initialization failed"))
			Expect(err).To(Not(BeNil()))
		})

		It("should fail to load vms: ", func() {
			loadVM = func(v2vv1.VirtualMachineImportSourceSpec) error {
				return fmt.Errorf("Not loaded")
			}

			err := reconciler.fetchVM(instance, mock)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to fetch resource mapping: ", func() {
			getResourceMapping = func(namespacedName types.NamespacedName) (*v2vv1.ResourceMapping, error) {
				return nil, fmt.Errorf("Not there")
			}
			instance.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{}

			err := reconciler.fetchVM(instance, mock)

			Expect(err).To(Not(BeNil()))
		})

		It("should init provider: ", func() {
			instance.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{}

			err := reconciler.fetchVM(instance, mock)

			Expect(err).To(BeNil())
		})

	})

	Describe("validate name", func() {
		It("should fail with a target VM name that is provided but empty: ", func() {
			emptyName := ""
			instance.Spec.TargetVMName = &emptyName
			err := validateName(instance, "ignored")
			Expect(err).To(HaveOccurred())
		})

		It("should fail with a source VM name that is empty if no target VM is provided: ", func() {
			emptyName := ""
			instance.Spec.TargetVMName = nil
			err := validateName(instance, emptyName)
			Expect(err).To(HaveOccurred())
		})

		It("should fail with a target VM name that contains invalid characters: ", func() {
			invalidName := "#$%^&*()"
			instance.Spec.TargetVMName = &invalidName
			err := validateName(instance, "ignored")
			Expect(err).To(HaveOccurred())
		})

		It("should fail with a source VM name that contains invalid characters: ", func() {
			invalidName := "#$%^&*()"
			instance.Spec.TargetVMName = nil
			err := validateName(instance, invalidName)
			Expect(err).To(HaveOccurred())
		})

		It("should fail with a target VM name that is longer than 63 characters: ", func() {
			longName := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"
			instance.Spec.TargetVMName = &longName
			err := validateName(instance, "ignored")
			Expect(err).To(HaveOccurred())
		})

		It("should fail with a source VM name that is longer than 63 characters: ", func() {
			longName := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"
			instance.Spec.TargetVMName = nil
			err := validateName(instance, longName)
			Expect(err).To(HaveOccurred())
		})

		It("should succeed with a target VM name that is 63 characters long: ", func() {
			name := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk"
			instance.Spec.TargetVMName = &name
			err := validateName(instance, "ignored")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should succeed with a source VM name that is 63 characters long: ", func() {
			name := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk"
			instance.Spec.TargetVMName = nil
			err := validateName(instance, name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("validate uniqueness", func() {
		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1.VirtualMachineImportOvirtSourceSpec{}
			instance.Name = "test"
			instance.Namespace = "test"
			mock = &mockProvider{}
		})

		It("should fail if the target VM already exists in the namespace", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					obj.(*kubevirtv1.VirtualMachine).Spec = kubevirtv1.VirtualMachineSpec{}
				}
				return nil
			}

			unique, err := reconciler.validateUniqueness(instance, "vm-exists")
			Expect(unique).To(BeFalse())
			Expect(err).To(BeNil())
		})

		It("should succeed if the target VM doesn't already exists in the namespace", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return errors.NewNotFound(schema.GroupResource{}, "")
			}

			unique, err := reconciler.validateUniqueness(instance, "not-found")
			Expect(unique).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})

	Describe("validate step", func() {
		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1.VirtualMachineImportOvirtSourceSpec{}
			instance.Spec.ResourceMapping = &v2vv1.ObjectIdentifier{}
			instance.Name = "test"
			instance.Namespace = "test"
			targetVMName := "valid-target-name"
			instance.Spec.TargetVMName = &targetVMName
			mock = &mockProvider{}

			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{}
				case *v2vv1.ResourceMapping:
					obj.(*v2vv1.ResourceMapping).Spec = v2vv1.ResourceMappingSpec{}
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *kubevirtv1.VirtualMachine:
					return errors.NewNotFound(schema.GroupResource{}, "")

				}
				return nil
			}
		})

		It("should succeed to validate: ", func() {
			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(BeNil())
			Expect(validated).To(Equal(true))
		})

		It("should fail to validate: ", func() {
			validate = func() ([]v2vv1.VirtualMachineImportCondition, error) {
				return nil, fmt.Errorf("Failed")
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should not validate again: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					return fmt.Errorf("Let's make it fail if it tries to validate")
				case *v2vv1.ResourceMapping:
					obj.(*v2vv1.ResourceMapping).Spec = v2vv1.ResourceMappingSpec{}
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *kubevirtv1.VirtualMachine:
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}

			conditions := []v2vv1.VirtualMachineImportCondition{}
			conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1.Valid,
			})
			conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1.MappingRulesVerified,
			})
			instance.Status.Conditions = conditions

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(BeNil())
			Expect(validated).To(Equal(true))
		})

		It("should fail to update conditions: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					return fmt.Errorf("Not found")
				case *v2vv1.ResourceMapping:
					obj.(*v2vv1.ResourceMapping).Spec = v2vv1.ResourceMappingSpec{}
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *kubevirtv1.VirtualMachine:
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should fail update vmimport with conditions: ", func() {
			update = func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
				return fmt.Errorf("Not modified")
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should fail with error conditions: ", func() {
			message := "message"
			validate = func() ([]v2vv1.VirtualMachineImportCondition, error) {
				conditions := []v2vv1.VirtualMachineImportCondition{}
				conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
					Status:  corev1.ConditionFalse,
					Message: &message,
				})
				return conditions, nil
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(BeNil())
			Expect(validated).To(Equal(false))
		})
	})

	Describe("createVM step", func() {
		var (
			mapper *mockMapper
		)
		BeforeEach(func() {
			findTemplate = func() (*oapiv1.Template, error) {
				return &oapiv1.Template{}, nil
			}
			processTemplate = func(template *oapiv1.Template, name *string, namespace string) (*kubevirtv1.VirtualMachine, error) {
				return &kubevirtv1.VirtualMachine{}, nil
			}
			mapper = &mockMapper{}
			targetName := "test"
			instance.Spec.TargetVMName = &targetName
		})

		It("should fail to find a template: ", func() {
			templateError := fmt.Errorf("Not found")
			findTemplate = func() (*oapiv1.Template, error) {
				return nil, templateError
			}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch vmImport := obj.(type) {
				case *v2vv1.VirtualMachineImport:
					vmImport.Annotations = map[string]string{"vmimport.v2v.kubevirt.io/source-vm-initial-state": "down"}
					vmImport.Spec = v2vv1.VirtualMachineImportSpec{}
				}
				return nil
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
			Expect(err).To(BeEquivalentTo(templateError))
		})

		It("should fail to process template: ", func() {
			templateProcessingError := fmt.Errorf("Failed")
			processTemplate = func(template *oapiv1.Template, name *string, namespace string) (*kubevirtv1.VirtualMachine, error) {
				return nil, templateProcessingError
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
			Expect(err).To(BeEquivalentTo(templateProcessingError))
		})

		It("should fail to update condition: ", func() {
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
		})

		It("should fail to create vm: ", func() {
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("Not created")
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
		})

		It("should fail to create vm and cleanup: ", func() {
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("Not created")
			}
			cleanUp = func() error {
				return fmt.Errorf("Failed")
			}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{}
					obj.(*v2vv1.VirtualMachineImport).Annotations = map[string]string{sourceVMInitialState: string(provider.VMStatusUp)}
				}
				return nil
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
		})

		It("should fail to modify vm name: ", func() {
			counter := 2
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					counter--
					if counter == 0 {
						return fmt.Errorf("Not modified")
					}
				}
				return nil
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
		})

		It("should fail to modify progress: ", func() {
			counter := 2
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					counter--
					if counter == 0 {
						return fmt.Errorf("Not modified")
					}
				}
				return nil
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(Not(BeNil()))
		})

		It("should succeed to create vm: ", func() {
			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(BeNil())
		})

		It("should succeed to create vm with description: ", func() {
			instance.Annotations = map[string]string{AnnPropagate: `{"description": "My description"}`}
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					Expect(obj.(*kubevirtv1.VirtualMachine).Annotations["description"], "My description")
				}
				return nil
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(BeNil())
		})

		It("should succeed to create vm with tracking label: ", func() {
			instance.Labels = map[string]string{TrackingLabel: "My tracker"}
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					Expect(obj.(*kubevirtv1.VirtualMachine).Labels[TrackingLabel], "My tracker")
				}
				return nil
			}

			name, err := reconciler.createVM(mock, instance, mapper)

			Expect(name).To(Equal(""))
			Expect(err).To(BeNil())
		})
	})

	Describe("startVM step", func() {

		BeforeEach(func() {
			shouldStart := true
			instance.Spec.StartVM = &shouldStart
			conditions := []v2vv1.VirtualMachineImportCondition{}
			reason := string(v2vv1.VirtualMachineReady)
			conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1.Succeeded,
				Reason: &reason,
			})
			instance.Status.Conditions = conditions
		})

		It("should not start vm: ", func() {
			instance.Status.Conditions = []v2vv1.VirtualMachineImportCondition{}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(BeNil())
		})

		It("should fail to start: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Not found")
			}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update progress: ", func() {
			fetch := 0
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				fetch++
				if fetch == 1 {
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update running: ", func() {
			fetch := 0
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				fetch++
				if fetch == 1 {
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}
			counter := 2
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update condition: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachineInstance:
					obj.(*kubevirtv1.VirtualMachineInstance).Status.Phase = kubevirtv1.Running
				}
				return nil
			}
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update progress after start: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachineInstance:
					obj.(*kubevirtv1.VirtualMachineInstance).Status.Phase = kubevirtv1.Running
				}
				return nil
			}
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should cleanup after start: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachineInstance:
					obj.(*kubevirtv1.VirtualMachineInstance).Status.Phase = kubevirtv1.Running
				case *kubevirtv1.VirtualMachine:
					volumes := []kubevirtv1.Volume{}
					volumes = append(volumes, kubevirtv1.Volume{
						Name: "test",
						VolumeSource: kubevirtv1.VolumeSource{
							DataVolume: &kubevirtv1.DataVolumeSource{
								Name: "test",
							},
						},
					})
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: volumes,
						},
					}
					refs := []v1.OwnerReference{}
					isController := true
					refs = append(refs, v1.OwnerReference{
						Controller: &isController,
					})
					obj.(*kubevirtv1.VirtualMachine).ObjectMeta.OwnerReferences = refs
				case *cdiv1.DataVolume:
					refs := []v1.OwnerReference{}
					isController := true
					refs = append(refs, v1.OwnerReference{
						Controller: &isController,
					})
					obj.(*cdiv1.DataVolume).ObjectMeta.OwnerReferences = refs
				}
				return nil
			}

			_, err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(BeNil())
		})
	})

	Describe("createDataVolumes step", func() {
		var (
			dv     cdiv1.DataVolume
			mapper provider.Mapper
		)
		BeforeEach(func() {
			update = func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
				return nil
			}
			mapper = &mockMapper{}
			dv = cdiv1.DataVolume{}
		})

		It("should fail to update status: ", func() {
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}
			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update progress: ", func() {
			counter := 2
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}

			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to create dv: ", func() {
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("Not created")
			}
			counter := 3
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}
			dv = cdiv1.DataVolume{}
			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update failure: ", func() {
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("Not created")
			}
			cleanUp = func() error {
				return fmt.Errorf("Failed")
			}
			dv = cdiv1.DataVolume{}

			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to add owner: ", func() {
			counter := 3
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}
			dv = cdiv1.DataVolume{}

			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update data volumes: ", func() {
			counter := 2
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}
			dv = cdiv1.DataVolume{}

			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update virtual machine: ", func() {
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					return fmt.Errorf("Not modified")
				}
				return nil
			}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: []kubevirtv1.Volume{},
						},
					}
				}
				return nil
			}

			dv = cdiv1.DataVolume{}

			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should succeed to create data volumes: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: []kubevirtv1.Volume{},
						},
					}
				}
				return nil
			}
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return nil
			}

			annotations := make(map[string]string)
			annotations[AnnDVNetwork] = "annDVNetwork"
			annotations[AnnDVMultusNetwork] = "annDVMultusNetwork"
			instance.SetAnnotations(annotations)

			dv := cdiv1.DataVolume{}
			_, err := reconciler.createDataVolume(mock, mapper, instance, &dv, vmName)

			Expect(err).To(BeNil())

			// ensure that DV transfer network annotations were propagated from VMI to DVs
			Expect(dv.Annotations).ToNot(BeNil())
			Expect(dv.Annotations[AnnDVNetwork]).To(Equal("annDVNetwork"))
			Expect(dv.Annotations[AnnDVMultusNetwork]).To(Equal("annDVMultusNetwork"))
		})
	})

	Describe("importDisks step", func() {
		var (
			mockMap *mockMapper
			vmName  types.NamespacedName
		)

		BeforeEach(func() {
			mockMap = &mockMapper{}
			vmName = types.NamespacedName{Name: "test", Namespace: "default"}
			mapDisks = func() (map[string]cdiv1.DataVolume, error) {
				return map[string]cdiv1.DataVolume{
					"test": cdiv1.DataVolume{},
				}, nil
			}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					volumes := []kubevirtv1.Volume{}
					volumes = append(volumes, kubevirtv1.Volume{
						Name: "test",
						VolumeSource: kubevirtv1.VolumeSource{
							DataVolume: &kubevirtv1.DataVolumeSource{
								Name: "test",
							},
						},
					})
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: volumes,
						},
					}
					refs := []v1.OwnerReference{}
					isController := true
					refs = append(refs, v1.OwnerReference{
						Controller: &isController,
					})
					obj.(*kubevirtv1.VirtualMachine).ObjectMeta.OwnerReferences = refs
				}
				return nil
			}

		})

		It("should do nothing: ", func() {
			mapDisks = func() (map[string]cdiv1.DataVolume, error) {
				return map[string]cdiv1.DataVolume{}, nil
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
		})

		It("should fail to create new dv: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("Not created")
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should create new dv with tracking label: ", func() {
			instance.Labels = map[string]string{TrackingLabel: "My tracker"}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					Expect(obj.(*cdiv1.DataVolume).Labels[TrackingLabel], "My tracker")
				}
				return nil
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
		})

		It("should not find a dv: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					return fmt.Errorf("Something went wrong")
				}
				return nil
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to manage state: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					obj.(*cdiv1.DataVolume).Status = cdiv1.DataVolumeStatus{
						Phase: cdiv1.Succeeded,
					}
				}
				return nil
			}
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should not fail for pending state: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					obj.(*cdiv1.DataVolume).Status = cdiv1.DataVolumeStatus{
						Phase: cdiv1.Pending,
					}
				}
				return nil
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
		})

		It("should cleanup: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					obj.(*cdiv1.DataVolume).Status = cdiv1.DataVolumeStatus{
						Phase: cdiv1.Succeeded,
					}
				case *kubevirtv1.VirtualMachine:
					obj.(*kubevirtv1.VirtualMachine).Spec = kubevirtv1.VirtualMachineSpec{
						Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtv1.VirtualMachineInstanceSpec{
								Volumes: []kubevirtv1.Volume{},
							},
						},
					}
				}
				return nil
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
		})

		It("import disk should succeed even if DV failed : ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					obj.(*cdiv1.DataVolume).Status = cdiv1.DataVolumeStatus{
						Phase: cdiv1.Failed,
					}
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{}
					obj.(*v2vv1.VirtualMachineImport).Annotations = map[string]string{sourceVMInitialState: string(provider.VMStatusDown)}
				}
				return nil
			}

			_, err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
		})
	})

	Describe("convertGuest step", func() {
		var (
			prov   *mockProvider
			vmName types.NamespacedName
			pod    *corev1.Pod
			mapper *mockMapper
		)

		BeforeEach(func() {
			prov = &mockProvider{}
			mapper = &mockMapper{}
			vmName = types.NamespacedName{Name: "test", Namespace: "default"}

			pod = &corev1.Pod{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			}

			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					volumes := []kubevirtv1.Volume{}
					volumes = append(volumes, kubevirtv1.Volume{
						Name: "test",
						VolumeSource: kubevirtv1.VolumeSource{
							DataVolume: &kubevirtv1.DataVolumeSource{
								Name: "test",
							},
						},
					})
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: volumes,
						},
					}
					refs := []v1.OwnerReference{}
					isController := true
					refs = append(refs, v1.OwnerReference{
						Controller: &isController,
					})
					obj.(*kubevirtv1.VirtualMachine).ObjectMeta.OwnerReferences = refs
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{}
					obj.(*v2vv1.VirtualMachineImport).Annotations = map[string]string{sourceVMInitialState: string(provider.VMStatusUp)}
				case *corev1.Pod:
					obj = pod
				}
				return nil
			}
			needsGuestConversion = func() bool {
				return true
			}
			getGuestConversionPod = func() (*corev1.Pod, error) {
				return nil, nil
			}
			launchGuestConversionPod = func() (*corev1.Pod, error) {
				return pod, nil
			}
		})

		It("should return false with no error when the pod is pending", func() {
			pod.Status.Phase = corev1.PodPending

			done, err := reconciler.convertGuest(prov, instance, mapper, vmName)
			Expect(err).To(BeNil())
			Expect(done).To(BeFalse())
		})

		It("should return false with no error when the pod is running", func() {
			pod.Status.Phase = corev1.PodRunning

			done, err := reconciler.convertGuest(prov, instance, mapper, vmName)
			Expect(err).To(BeNil())
			Expect(done).To(BeFalse())
		})

		It("should return false with no error when the pod is failed", func() {
			pod.Status.Phase = corev1.PodFailed

			done, err := reconciler.convertGuest(prov, instance, mapper, vmName)
			Expect(err).To(BeNil())
			Expect(done).To(BeFalse())
		})

		It("should return true with no error when the pod is successful", func() {
			pod.Status.Phase = corev1.PodSucceeded

			done, err := reconciler.convertGuest(prov, instance, mapper, vmName)
			Expect(err).To(BeNil())
			Expect(done).To(BeTrue())
		})
	})

	Describe("afterSuccess and afterFailure steps", func() {
		var (
			config *v2vv1.VirtualMachineImport
		)
		BeforeEach(func() {
			config = &v2vv1.VirtualMachineImport{}
			config.Spec = v2vv1.VirtualMachineImportSpec{
				Source: v2vv1.VirtualMachineImportSourceSpec{
					Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
				},
			}
			conditions := []v2vv1.VirtualMachineImportCondition{}
			conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1.Valid,
			})
			conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1.MappingRulesVerified,
			})
			config.Status.Conditions = conditions
			name := "test"
			config.Spec.TargetVMName = &name
			vmName = types.NamespacedName{Name: "test", Namespace: "default"}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					volumes := []kubevirtv1.Volume{}
					volumes = append(volumes, kubevirtv1.Volume{
						Name: "test",
						VolumeSource: kubevirtv1.VolumeSource{
							DataVolume: &kubevirtv1.DataVolumeSource{
								Name: "test",
							},
						},
					})
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: volumes,
						},
					}
					refs := []v1.OwnerReference{}
					isController := true
					refs = append(refs, v1.OwnerReference{
						Controller: &isController,
					})
					obj.(*kubevirtv1.VirtualMachine).ObjectMeta.OwnerReferences = refs
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{}
					obj.(*v2vv1.VirtualMachineImport).Annotations = map[string]string{sourceVMInitialState: string(provider.VMStatusUp)}
				}
				return nil
			}
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).DeepCopyInto(config)
				default:
					return nil
				}

				return nil
			}
		})
		It("should increment success counter", func() {
			counterValueBefore := getCounterSuccessful()
			durationSamplesBefore := getCountDurationSuccessful()

			err := reconciler.afterSuccess(vmName, &mockProvider{}, config)

			Expect(err).To(BeNil())
			counterValueAfter := getCounterSuccessful()
			Expect(counterValueAfter).To(Equal(counterValueBefore + 1))
			Expect(config.Finalizers).To(BeNil())
			durationSamplesAfter := getCountDurationSuccessful()
			Expect(durationSamplesAfter).To(Equal(durationSamplesBefore + 1))
		})
		It("should increment failed counter", func() {
			counterValueBefore := getCounterFailed()
			durationSamplesBefore := getCountDurationFailed()

			err := reconciler.afterFailure(&mockProvider{}, config)

			Expect(err).To(BeNil())
			counterValueAfter := getCounterFailed()
			Expect(counterValueAfter).To(Equal(counterValueBefore + 1))
			Expect(config.Finalizers).To(BeNil())
			durationSamplesAfter := getCountDurationFailed()
			Expect(durationSamplesAfter).To(Equal(durationSamplesBefore + 1))
		})
	})

	Describe("Reconcile step", func() {

		var (
			request reconcile.Request
		)

		BeforeEach(func() {
			request = reconcile.Request{}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					conditions := []v2vv1.VirtualMachineImportCondition{}
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.Valid,
					})
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.MappingRulesVerified,
					})
					obj.(*v2vv1.VirtualMachineImport).Status.Conditions = conditions
					annotations := map[string]string{"vmimport.v2v.kubevirt.io/source-vm-initial-state": "up"}
					obj.(*v2vv1.VirtualMachineImport).Annotations = annotations
					name := "test"
					obj.(*v2vv1.VirtualMachineImport).Spec.TargetVMName = &name
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				}
				return nil
			}
			getVM = func(id *string, name *string, cluster *string, clusterID *string) (interface{}, error) {
				return newVM(), nil
			}
			stopVM = func(id string) error {
				return nil
			}
			list = func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
				switch list.(type) {
				case *corev1.SecretList:
					list.(*corev1.SecretList).Items = []corev1.Secret{
						corev1.Secret{},
					}
				case *corev1.ConfigMapList:
					list.(*corev1.ConfigMapList).Items = []corev1.ConfigMap{
						corev1.ConfigMap{},
					}
				}
				return nil
			}
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return nil
			}
			rec := record.NewFakeRecorder(3)
			reconciler.recorder = rec
		})

		AfterEach(func() {
			if reconciler != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
				reconciler = nil
			}
		})

		It("should fail to find vm import: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return errors.NewNotFound(schema.GroupResource{}, "")
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to get vm import: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Not found")
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to create a provider: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: nil,
						},
					}
				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to initiate a provider of new request due to missing secret: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
				case *corev1.Secret:
					return fmt.Errorf("Not found")

				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To((BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to initiate a provider of request in progress: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					obj.(*v2vv1.VirtualMachineImport).Annotations = map[string]string{"vmimport.v2v.kubevirt.io/progress": "10"}
				case *corev1.Secret:
					return fmt.Errorf("Not found")

				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to validate: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:

					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *kubevirtv1.VirtualMachine:
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}
			getVM = func(id *string, name *string, cluster *string, clusterID *string) (interface{}, error) {
				return ovirtsdk.NewVmBuilder().Name("myvm").MustBuild(), nil
			}

			result, err := reconciler.Reconcile(request)

			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("import blocked"))
			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: requeueAfterValidationFailureTime}))
		})

		It("should fail to stop vm: ", func() {
			stopVM = func(id string) error {
				return fmt.Errorf("Not so fast")
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to create mapper: ", func() {
			list = func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
				return fmt.Errorf("Not found")
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to create vm: ", func() {
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				return fmt.Errorf("Not created")
			}

			result, err := reconciler.Reconcile(request)

			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("Error while creating virtual machine"))
			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to import disks: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					conditions := []v2vv1.VirtualMachineImportCondition{}
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.Valid,
					})
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.MappingRulesVerified,
					})
					obj.(*v2vv1.VirtualMachineImport).Status.Conditions = conditions
					annotations := map[string]string{"vmimport.v2v.kubevirt.io/source-vm-initial-state": "up"}
					obj.(*v2vv1.VirtualMachineImport).Annotations = annotations
					name := "test"
					obj.(*v2vv1.VirtualMachineImport).Spec.TargetVMName = &name
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *cdiv1.DataVolume:
					return errors.NewNotFound(schema.GroupResource{}, "")
				}
				return nil
			}
			counter := 2
			create = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not created")
				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("started"))
			event = <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("creation failed"))

			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to import disks when VM update fails: ", func() {
			vmImportGetCounter := 10
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					vmImport := obj.(*v2vv1.VirtualMachineImport)
					vmImport.Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					conditions := []v2vv1.VirtualMachineImportCondition{}
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.Valid,
					})
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.MappingRulesVerified,
					})
					vmImport.Status.Conditions = conditions
					vmImport.Status.DataVolumes = []v2vv1.DataVolumeItem{}
					name := "test"
					vmImportGetCounter--
					if vmImportGetCounter == 0 {
						vmImport.Status.TargetVMName = name
					}
					vmImport.Spec.TargetVMName = &name
					vmImport.Annotations = map[string]string{"vmimport.v2v.kubevirt.io/source-vm-initial-state": "down"}

				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *cdiv1.DataVolume:
					return errors.NewNotFound(schema.GroupResource{}, "")
				case *kubevirtv1.VirtualMachine:
					obj.(*kubevirtv1.VirtualMachine).Spec.Template = &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Volumes: []kubevirtv1.Volume{},
						},
					}
				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("ImportScheduled"))
			event = <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("ImportInProgress"))

			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: SlowReQ}))
		})

		It("should fail to start vm: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					conditions := []v2vv1.VirtualMachineImportCondition{}
					reason := string(v2vv1.VirtualMachineReady)
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Reason: &reason,
						Type:   v2vv1.Succeeded,
					})
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.Valid,
					})
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1.MappingRulesVerified,
					})
					obj.(*v2vv1.VirtualMachineImport).Status.Conditions = conditions
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					start := true
					obj.(*v2vv1.VirtualMachineImport).Spec.StartVM = &start
					name := "test"
					obj.(*v2vv1.VirtualMachineImport).Spec.TargetVMName = &name
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				case *kubevirtv1.VirtualMachineInstance:
					return fmt.Errorf("Not found")
				}
				return nil
			}

			result, err := reconciler.Reconcile(request)
			Expect(err).To(Not(BeNil()))
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should succeed: ", func() {
			result, err := reconciler.Reconcile(request)

			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: SlowReQ}))
		})

		It("should exit early if vm condition is succeed: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1.VirtualMachineImport:
					conditions := []v2vv1.VirtualMachineImportCondition{}
					reason := string(v2vv1.VirtualMachineRunning)
					conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Reason: &reason,
						Type:   v2vv1.Succeeded,
					})
					obj.(*v2vv1.VirtualMachineImport).Status.Conditions = conditions
					obj.(*v2vv1.VirtualMachineImport).Spec = v2vv1.VirtualMachineImportSpec{
						Source: v2vv1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					start := true
					obj.(*v2vv1.VirtualMachineImport).Spec.StartVM = &start
					name := "test"
					obj.(*v2vv1.VirtualMachineImport).Spec.TargetVMName = &name
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		Describe("Cancelled import", func() {
			var (
				config *v2vv1.VirtualMachineImport
			)
			BeforeEach(func() {
				config = &v2vv1.VirtualMachineImport{}
				config.Annotations = map[string]string{"vmimport.v2v.kubevirt.io/source-vm-initial-state": "up"}
				config.Spec = v2vv1.VirtualMachineImportSpec{
					Source: v2vv1.VirtualMachineImportSourceSpec{
						Ovirt: &v2vv1.VirtualMachineImportOvirtSourceSpec{},
					},
				}
				conditions := []v2vv1.VirtualMachineImportCondition{}
				conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
					Status: corev1.ConditionTrue,
					Type:   v2vv1.Valid,
				})
				conditions = append(conditions, v2vv1.VirtualMachineImportCondition{
					Status: corev1.ConditionTrue,
					Type:   v2vv1.MappingRulesVerified,
				})
				config.Status.Conditions = conditions
				name := "test"
				config.Spec.TargetVMName = &name
				get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch obj.(type) {
					case *v2vv1.VirtualMachineImport:
						config.DeepCopyInto(obj.(*v2vv1.VirtualMachineImport))
					case *corev1.Secret:
						obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
					}
					return nil
				}
				statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
					switch obj.(type) {
					case *v2vv1.VirtualMachineImport:
						obj.(*v2vv1.VirtualMachineImport).DeepCopyInto(config)
					default:
						return nil
					}

					return nil
				}
			})

			It("should increment counter for not in progress import: ", func() {
				config.SetDeletionTimestamp(&v1.Time{})

				counterValueBefore := getCounterCancelled()
				durationSamplesBefore := getCountDurationCancelled()

				result, err := reconciler.Reconcile(request)

				Expect(err).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))

				counterValueAfter := getCounterCancelled()
				Expect(counterValueAfter).To(Equal(counterValueBefore + 1))
				Expect(config.Finalizers).To(BeNil())
				durationSamplesAfter := getCountDurationCancelled()
				Expect(durationSamplesAfter).To(Equal(durationSamplesBefore + 1))
			})

			It("should not increment counter for done import: ", func() {
				config.SetDeletionTimestamp(&v1.Time{})
				config.Annotations = make(map[string]string)
				config.Annotations[AnnCurrentProgress] = progressDone

				counterValueBefore := getCounterCancelled()
				durationSamplesBefore := getCountDurationCancelled()

				result, err := reconciler.Reconcile(request)

				Expect(err).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))

				counterValueAfter := getCounterCancelled()
				Expect(counterValueAfter).To(Equal(counterValueBefore))
				Expect(config.Annotations[AnnCurrentProgress]).To(Equal(progressDone))
				Expect(config.Finalizers).To(BeNil())
				durationSamplesAfter := getCountDurationCancelled()
				Expect(durationSamplesAfter).To(Equal(durationSamplesBefore))
			})

			It("should increment counter for in progress import: ", func() {
				counterValueBefore := getCounterCancelled()
				durationSamplesBefore := getCountDurationCancelled()
				// First call Reconcile to make it in progress
				result, err := reconciler.Reconcile(request)

				Expect(err).To(BeNil())

				Expect(result).To(Equal(reconcile.Result{RequeueAfter: SlowReQ}))
				Expect(config.Finalizers).To(Not(BeNil()))
				Expect(config.Annotations[AnnCurrentProgress]).To(Not(BeNil()))

				// Now make the resource deleted and call Reocncile
				config.SetDeletionTimestamp(&v1.Time{})

				result, err = reconciler.Reconcile(request)

				Expect(err).To(BeNil())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(config.Finalizers).To(BeNil())
				counterValueAfter := getCounterCancelled()
				Expect(counterValueAfter).To(Equal(counterValueBefore + 1))
				durationSamplesAfter := getCountDurationCancelled()
				Expect(durationSamplesAfter).To(Equal(durationSamplesBefore + 1))
			})
		})
	})
})

var _ = Describe("Disks import progress", func() {
	table.DescribeTable("Percentage count",
		func(disks map[string]float64, expected string) {
			result := disksImportProgress(disks, float64(len(disks)))
			Expect(result).To(Equal(expected))
		},
		table.Entry("No progress", map[string]float64{"1": 0.0}, "10"),
		table.Entry("Two disks no progress", map[string]float64{"1": 0.0, "2": 0.0}, "10"),
		table.Entry("Two disks done progress", map[string]float64{"1": 100, "2": 100}, "75"),
		table.Entry("Two disks half done", map[string]float64{"1": 50, "2": 50}, "42"),
		table.Entry("Two disks one done", map[string]float64{"1": 50, "2": 100}, "58"),
		table.Entry("Done progress", map[string]float64{"1": 100}, "75"),
	)
})

func NewReconciler(client client.Client, finder mappings.ResourceFinder, scheme *runtime.Scheme, ownerreferencesmgr ownerreferences.OwnerReferenceManager, factory pclient.Factory, recorder record.EventRecorder, controller controller.Controller, ctrlConfigProvider ctrlConfig.ControllerConfigProvider) *ReconcileVirtualMachineImport {
	return &ReconcileVirtualMachineImport{
		client:                 client,
		apiReader:              client,
		resourceMappingsFinder: finder,
		scheme:                 scheme,
		ownerreferencesmgr:     ownerreferencesmgr,
		factory:                factory,
		recorder:               recorder,
		controller:             controller,
		ctrlConfigProvider:     ctrlConfigProvider,
	}
}

type mockClient struct{}

type mockFinder struct{}

type mockProvider struct{}

type mockMapper struct{}

type mockFactory struct{}

type mockController struct{}

type mockControllerConfigProvider struct{}

type mockOvirtClient struct{}

type mockVmwareClient struct{}

// Create implements client.Client
func (c *mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return create(ctx, obj)
}

// Update implements client.Client
func (c *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return update(ctx, obj)
}

// Delete implements client.Client
func (c *mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return nil
}

// DeleteAllOf implements client.Client
func (c *mockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

// Patch implements client.Client
func (c *mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return statusPatch(ctx, obj, patch)
}

// Get implements client.Client
func (c *mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return get(ctx, key, obj)
}

// List implements client.Client
func (c *mockClient) List(ctx context.Context, objectList runtime.Object, opts ...client.ListOption) error {
	return list(ctx, objectList)
}

// Status implements client.StatusClient
func (c *mockClient) Status() client.StatusWriter {
	return c
}

// GetResourceMapping implements ResourceFinder.GetResourceMapping
func (m *mockFinder) GetResourceMapping(namespacedName types.NamespacedName) (*v2vv1.ResourceMapping, error) {
	return getResourceMapping(namespacedName)
}

// Init implements Provider.Init
func (p *mockProvider) Init(secret *corev1.Secret, instance *v2vv1.VirtualMachineImport) error {
	return pinit(secret, instance)
}

// TestConnection implements Provider.TestConnection
func (p *mockProvider) TestConnection() error {
	return nil
}

// ValidateDiskStatus return true if disk is valid
func (p *mockProvider) ValidateDiskStatus(string) (bool, error) {
	return true, nil
}

// Close implements Provider.Close
func (p *mockProvider) Close() {}

// LoadVM implements Provider.LoadVM
func (p *mockProvider) LoadVM(spec v2vv1.VirtualMachineImportSourceSpec) error {
	return loadVM(spec)
}

// PrepareResourceMapping implements Provider.PrepareResourceMapping
func (p *mockProvider) PrepareResourceMapping(*v2vv1.ResourceMappingSpec, v2vv1.VirtualMachineImportSourceSpec) {
}

// LoadVM implements Provider.LoadVM
func (p *mockProvider) Validate() ([]v2vv1.VirtualMachineImportCondition, error) {
	return validate()
}

// StopVM implements Provider.StopVM
func (p *mockProvider) StopVM(cr *v2vv1.VirtualMachineImport, client rclient.Client) error {
	return nil
}

// UpdateVM implements Provider.UpdateVM
func (p *mockProvider) UpdateVM(vmSpec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume) error {
	return nil
}

// CreateMapper implements Provider.CreateMapper
func (p *mockProvider) CreateMapper() (provider.Mapper, error) {
	return nil, nil
}

// GetVMStatus implements Provider.GetVMStatus
func (p *mockProvider) GetVMStatus() (provider.VMStatus, error) {
	return getVMStatus()
}

// GetVMName implements Provider.GetVMName
func (p *mockProvider) GetVMName() (string, error) {
	return "", nil
}

// StartVM implements Provider.StartVM
func (p *mockProvider) StartVM() error {
	return nil
}

// CleanUp implements Provider.CleanUp
func (p *mockProvider) CleanUp(failure bool, cr *v2vv1.VirtualMachineImport, client rclient.Client) error {
	return cleanUp()
}

// FindTemplate implements Provider.FindTemplate
func (p *mockProvider) FindTemplate() (*oapiv1.Template, error) {
	return findTemplate()
}

// ProcessTemplate implements Provider.ProcessTemplate
func (p *mockProvider) ProcessTemplate(template *oapiv1.Template, name *string, namespace string) (*kubevirtv1.VirtualMachine, error) {
	return processTemplate(template, name, namespace)
}

func (p *mockProvider) NeedsGuestConversion() bool {
	return needsGuestConversion()
}

func (p *mockProvider) GetGuestConversionPod() (*corev1.Pod, error) {
	return getGuestConversionPod()
}

func (p *mockProvider) LaunchGuestConversionPod(_ *kubevirtv1.VirtualMachine, _ map[string]cdiv1.DataVolume) (*corev1.Pod, error) {
	return launchGuestConversionPod()
}

func (p *mockProvider) SupportsWarmMigration() bool {
	return supportsWarmMigration()
}

func (p *mockProvider) CreateVMSnapshot() (string, error) {
	return createVMSnapshot()
}

func (p *mockProvider) RemoveVMSnapshot(snapshotID string, removeChildren bool) error {
	return removeVMSnapshot(snapshotID, removeChildren)
}

// CreateEmptyVM implements Mapper.CreateEmptyVM
func (m *mockMapper) CreateEmptyVM(vmName *string) *kubevirtv1.VirtualMachine {
	return &kubevirtv1.VirtualMachine{}
}

// ResolveVMName implements Mapper.ResolveVMName
func (m *mockMapper) ResolveVMName(targetVMName *string) *string {
	return targetVMName
}

// MapVM implements Mapper.MapVM
func (m *mockMapper) MapVM(targetVMName *string, vmSpec *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	return vmSpec, nil
}

// MapDataVolumes implements Mapper.MapDataVolumes
func (m *mockMapper) MapDataVolumes(targetVMName *string, overhead cdiv1.FilesystemOverhead) (map[string]cdiv1.DataVolume, error) {
	return map[string]cdiv1.DataVolume{"123": {}}, nil
}

// MapDisks implements Mapper.MapDataVolumes
func (m *mockMapper) MapDisk(vmSpec *kubevirtv1.VirtualMachine, dv cdiv1.DataVolume) {
}

// RunningState implements Mapper.RunningState
func (m *mockMapper) RunningState() bool {
	return false
}

// NewOvirtClient implements Factory.NewOvirtClient
func (f *mockFactory) NewOvirtClient(dataMap map[string]string) (pclient.VMClient, error) {
	return &mockOvirtClient{}, nil
}

// NewVmwareClient implements Factory.NewVmwareClient
func (f *mockFactory) NewVmwareClient(dataMap map[string]string) (pclient.VMClient, error) {
	return &mockVmwareClient{}, nil
}

func (f *mockController) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	return nil
}

func (f *mockController) Start(stop <-chan struct{}) error {
	return nil
}

func (f *mockController) Reconcile(reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (c *mockOvirtClient) GetVM(id *string, name *string, cluster *string, clusterID *string) (interface{}, error) {
	return getVM(id, name, cluster, clusterID)
}

func (c *mockOvirtClient) StopVM(id string) error {
	return stopVM(id)
}

func (c *mockOvirtClient) StartVM(id string) error {
	return nil
}

func (c *mockOvirtClient) TestConnection() error {
	return nil
}

func (c *mockOvirtClient) Close() error {
	return nil
}

func (c *mockVmwareClient) GetVM(id *string, name *string, cluster *string, clusterID *string) (interface{}, error) {
	return getVM(id, name, cluster, clusterID)
}

func (c *mockVmwareClient) StopVM(id string) error {
	return stopVM(id)
}

func (c *mockVmwareClient) StartVM(id string) error {
	return nil
}

func (c *mockVmwareClient) Close() error {
	return nil
}

func (c *mockVmwareClient) TestConnection() error {
	return nil
}

func (c *mockControllerConfigProvider) GetConfig() (ctrlConfig.ControllerConfig, error) {
	return getCtrlConfig(), nil
}

func getSecret() []byte {
	contents := []byte(`{"apiUrl": "https://test", "username": "admin@internal", "password": "password", "caCert": "ABC"}`)
	secret, _ := yaml.JSONToYAML(contents)
	return secret
}

func newVM() *ovirtsdk.Vm {
	vm := ovirtsdk.Vm{}
	nicSlice := ovirtsdk.NicSlice{}
	nicSlice.SetSlice([]*ovirtsdk.Nic{&ovirtsdk.Nic{}})
	vm.SetNics(&nicSlice)
	diskAttachement := ovirtsdk.NewDiskAttachmentBuilder().
		Id("123").
		Disk(
			ovirtsdk.NewDiskBuilder().
				Id("disk-ID").
				Name("mydisk").
				Bootable(true).
				ProvisionedSize(1024).
				StorageDomain(
					ovirtsdk.NewStorageDomainBuilder().
						Name("mystoragedomain").MustBuild()).
				MustBuild()).MustBuild()
	daSlice := ovirtsdk.DiskAttachmentSlice{}
	daSlice.SetSlice([]*ovirtsdk.DiskAttachment{diskAttachement})
	vm.SetDiskAttachments(&daSlice)
	bios := ovirtsdk.NewBiosBuilder().
		Type(ovirtsdk.BIOSTYPE_Q35_SEA_BIOS).MustBuild()
	vm.SetBios(bios)
	cpu := ovirtsdk.NewCpuBuilder().
		Topology(
			ovirtsdk.NewCpuTopologyBuilder().
				Cores(1).
				Sockets(2).
				Threads(4).
				MustBuild()).MustBuild()
	vm.SetCpu(cpu)
	ha := ovirtsdk.NewHighAvailabilityBuilder().
		Enabled(true).
		MustBuild()
	vm.SetHighAvailability(ha)
	vm.SetMemory(1024)
	vm.SetMemoryPolicy(ovirtsdk.NewMemoryPolicyBuilder().
		Max(1024).MustBuild())
	gc := ovirtsdk.NewGraphicsConsoleBuilder().
		Name("testConsole").MustBuild()
	gcSlice := ovirtsdk.GraphicsConsoleSlice{}
	gcSlice.SetSlice([]*ovirtsdk.GraphicsConsole{gc})
	vm.SetGraphicsConsoles(&gcSlice)
	vm.SetPlacementPolicy(ovirtsdk.NewVmPlacementPolicyBuilder().
		Affinity(ovirtsdk.VMAFFINITY_MIGRATABLE).MustBuild())
	return &vm
}

func getCounterFailed() float64 {
	value, err := metrics.ImportMetrics.GetFailed()
	Expect(err).To(BeNil())
	return value
}

func getCounterCancelled() float64 {
	value, err := metrics.ImportMetrics.GetCancelled()
	Expect(err).To(BeNil())
	return value
}

func getCounterSuccessful() float64 {
	value, err := metrics.ImportMetrics.GetSuccessful()
	Expect(err).To(BeNil())
	return value
}

func getCountDurationFailed() uint64 {
	value, err := metrics.ImportMetrics.GetCountDurationFailed()
	Expect(err).To(BeNil())
	return value
}

func getCountDurationCancelled() uint64 {
	value, err := metrics.ImportMetrics.GetCountDurationCancelled()
	Expect(err).To(BeNil())
	return value
}

func getCountDurationSuccessful() uint64 {
	value, err := metrics.ImportMetrics.GetCountDurationSuccessful()
	Expect(err).To(BeNil())
	return value
}
