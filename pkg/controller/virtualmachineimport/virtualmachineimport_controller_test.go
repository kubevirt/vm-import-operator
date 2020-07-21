package virtualmachineimport

import (
	"context"
	"fmt"

	"github.com/kubevirt/vm-import-operator/pkg/config"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
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
	get                func(context.Context, client.ObjectKey, runtime.Object) error
	pinit              func(*corev1.Secret, *v2vv1alpha1.VirtualMachineImport) error
	loadVM             func(v2vv1alpha1.VirtualMachineImportSourceSpec) error
	getResourceMapping func(types.NamespacedName) (*v2vv1alpha1.ResourceMapping, error)
	validate           func() ([]v2vv1alpha1.VirtualMachineImportCondition, error)
	statusPatch        func(ctx context.Context, obj runtime.Object, patch client.Patch) error
	getVMStatus        func() (provider.VMStatus, error)
	findTemplate       func() (*oapiv1.Template, error)
	processTemplate    func(template *oapiv1.Template, name *string, namespace string) (*kubevirtv1.VirtualMachine, error)
	create             func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error
	cleanUp            func() error
	update             func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error
	mapDisks           func() (map[string]cdiv1.DataVolume, error)
	getVM              func(id *string, name *string, cluster *string, clusterID *string) (interface{}, error)
	stopVM             func(id string) error
	list               func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error
	getConfig          func() config.KubeVirtConfig
)
var _ = Describe("Reconcile steps", func() {
	var (
		reconciler *ReconcileVirtualMachineImport
		instance   *v2vv1alpha1.VirtualMachineImport
		vmName     types.NamespacedName
		mock       *mockProvider
	)

	BeforeEach(func() {
		instance = &v2vv1alpha1.VirtualMachineImport{}
		mockClient := &mockClient{}
		finder := &mockFinder{}
		scheme := runtime.NewScheme()
		factory := &mockFactory{}
		controller := &mockController{}
		kvConfigProviderMock := &mockKubeVirtConfigProvider{}
		scheme.AddKnownTypes(v2vv1alpha1.SchemeGroupVersion,
			&v2vv1alpha1.VirtualMachineImport{},
		)
		scheme.AddKnownTypes(cdiv1.SchemeGroupVersion,
			&cdiv1.DataVolume{},
		)
		scheme.AddKnownTypes(kubevirtv1.GroupVersion,
			&kubevirtv1.VirtualMachine{},
		)

		get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
			switch obj.(type) {
			case *v2vv1alpha1.VirtualMachineImport:
				obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{}
			case *v2vv1alpha1.ResourceMapping:
				obj.(*v2vv1alpha1.ResourceMapping).Spec = v2vv1alpha1.ResourceMappingSpec{}
			case *corev1.Secret:
				obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
			}
			return nil
		}
		statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
			return nil
		}
		validate = func() ([]v2vv1alpha1.VirtualMachineImportCondition, error) {
			return []v2vv1alpha1.VirtualMachineImportCondition{}, nil
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
		getConfig = func() config.KubeVirtConfig {
			return config.KubeVirtConfig{FeatureGates: "ImportWithoutTemplate"}
		}
		vmName = types.NamespacedName{Name: "test", Namespace: "default"}
		rec := record.NewFakeRecorder(2)

		reconciler = NewReconciler(mockClient, finder, scheme, ownerreferences.NewOwnerReferenceManager(mockClient), factory, kvConfigProviderMock, rec, controller)
	})

	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	Describe("Init steps", func() {
		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{}
			instance.Name = "test"
			instance.Namespace = "test"
		})

		It("should fail to create provider: ", func() {
			instance.Spec.Source.Ovirt = nil

			provider, err := reconciler.createProvider(instance)

			Expect(provider).To(BeNil())
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(Equal("Invalid source type. only Ovirt type is supported"))
		})

		It("should create provider: ", func() {
			provider, err := reconciler.createProvider(instance)

			Expect(provider).To(Not(BeNil()))
			Expect(err).To(BeNil())
		})
	})

	Describe("Create steps", func() {

		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{}
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
			pinit = func(*corev1.Secret, *v2vv1alpha1.VirtualMachineImport) error {
				return nil
			}
			loadVM = func(v2vv1alpha1.VirtualMachineImportSourceSpec) error {
				return nil
			}
			getResourceMapping = func(types.NamespacedName) (*v2vv1alpha1.ResourceMapping, error) {
				return &v2vv1alpha1.ResourceMapping{
					Spec: v2vv1alpha1.ResourceMappingSpec{},
				}, nil
			}
			instance.Spec.Source.Ovirt = &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{}
			instance.Name = "test"
			instance.Namespace = "test"

			mock = &mockProvider{}
		})

		It("should fail to connect: ", func() {
			pinit = func(*corev1.Secret, *v2vv1alpha1.VirtualMachineImport) error {
				return fmt.Errorf("Not connected")
			}

			msg, err := reconciler.initProvider(instance, mock)

			Expect(msg).To(Equal("Source provider initialization failed"))
			Expect(err).To(Not(BeNil()))
		})

		It("should fail to load vms: ", func() {
			loadVM = func(v2vv1alpha1.VirtualMachineImportSourceSpec) error {
				return fmt.Errorf("Not loaded")
			}

			err := reconciler.fetchVM(instance, mock)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to fetch resource mapping: ", func() {
			getResourceMapping = func(namespacedName types.NamespacedName) (*v2vv1alpha1.ResourceMapping, error) {
				return nil, fmt.Errorf("Not there")
			}
			instance.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{}

			err := reconciler.fetchVM(instance, mock)

			Expect(err).To(Not(BeNil()))
		})

		It("should init provider: ", func() {
			instance.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{}

			err := reconciler.fetchVM(instance, mock)

			Expect(err).To(BeNil())
		})

	})

	Describe("validate step", func() {
		BeforeEach(func() {
			instance.Spec.Source.Ovirt = &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{}
			instance.Spec.ResourceMapping = &v2vv1alpha1.ObjectIdentifier{}
			instance.Name = "test"
			instance.Namespace = "test"
			mock = &mockProvider{}
		})

		It("should succeed to validate: ", func() {
			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(BeNil())
			Expect(validated).To(Equal(true))
		})

		It("should fail to validate: ", func() {
			validate = func() ([]v2vv1alpha1.VirtualMachineImportCondition, error) {
				return nil, fmt.Errorf("Failed")
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should not validate again: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1alpha1.VirtualMachineImport:
					return fmt.Errorf("Let's make it fail if it tries to validate")
				case *v2vv1alpha1.ResourceMapping:
					obj.(*v2vv1alpha1.ResourceMapping).Spec = v2vv1alpha1.ResourceMappingSpec{}
				default:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				}
				return nil
			}

			conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
			conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1alpha1.Valid,
			})
			conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1alpha1.MappingRulesVerified,
			})
			instance.Status.Conditions = conditions

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(BeNil())
			Expect(validated).To(Equal(true))
		})

		It("should fail to update conditions: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1alpha1.VirtualMachineImport:
					return fmt.Errorf("Not found")
				case *v2vv1alpha1.ResourceMapping:
					obj.(*v2vv1alpha1.ResourceMapping).Spec = v2vv1alpha1.ResourceMappingSpec{}
				default:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				}
				return nil
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should fail update vmimport with conditions: ", func() {
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should fail with error conditions: ", func() {
			message := "message"
			validate = func() ([]v2vv1alpha1.VirtualMachineImportCondition, error) {
				conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
				conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
					Status:  corev1.ConditionFalse,
					Message: &message,
				})
				return conditions, nil
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(BeNil())
			Expect(validated).To(Equal(false))
		})

		It("should fail with vm status: ", func() {
			getVMStatus = func() (provider.VMStatus, error) {
				return "", fmt.Errorf("Not found")
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
		})

		It("should fail to store vm status: ", func() {
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				if obj.(*v2vv1alpha1.VirtualMachineImport).Annotations[sourceVMInitialState] == string(provider.VMStatusDown) {
					return fmt.Errorf("Not modified")
				}
				return nil
			}

			validated, err := reconciler.validate(instance, mock)

			Expect(err).To(Not(BeNil()))
			Expect(validated).To(Equal(true))
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
			getConfig = func() config.KubeVirtConfig {
				// Feature flag is not present
				return config.KubeVirtConfig{}
			}
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch vmImport := obj.(type) {
				case *v2vv1alpha1.VirtualMachineImport:
					vmImport.Annotations = map[string]string{"vmimport.v2v.kubevirt.io/source-vm-initial-state": "down"}
					vmImport.Spec = v2vv1alpha1.VirtualMachineImportSpec{}
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
			getConfig = func() config.KubeVirtConfig {
				// Feature flag is not present
				return config.KubeVirtConfig{}
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{}
					obj.(*v2vv1alpha1.VirtualMachineImport).Annotations = map[string]string{sourceVMInitialState: string(provider.VMStatusUp)}
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
				case *v2vv1alpha1.VirtualMachineImport:
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
			counter := 3
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				switch obj.(type) {
				case *v2vv1alpha1.VirtualMachineImport:
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
			conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
			reason := string(v2vv1alpha1.VirtualMachineReady)
			conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1alpha1.Succeeded,
				Reason: &reason,
			})
			instance.Status.Conditions = conditions
		})

		It("should not start vm: ", func() {
			instance.Status.Conditions = []v2vv1alpha1.VirtualMachineImportCondition{}

			err := reconciler.startVM(mock, instance, vmName)

			Expect(err).To(BeNil())
		})

		It("should fail to start: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Not found")
			}

			err := reconciler.startVM(mock, instance, vmName)

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

			err := reconciler.startVM(mock, instance, vmName)

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

			err := reconciler.startVM(mock, instance, vmName)

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

			err := reconciler.startVM(mock, instance, vmName)

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
			counter := 2
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}

			err := reconciler.startVM(mock, instance, vmName)

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

			err := reconciler.startVM(mock, instance, vmName)

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
			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

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

			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

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
			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

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

			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

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

			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update data volumes: ", func() {
			counter := 4
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
					return fmt.Errorf("Not modified")
				}
				return nil
			}
			dv = cdiv1.DataVolume{}

			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

			Expect(err).To(Not(BeNil()))
		})

		It("should fail to update virtual machine: ", func() {
			counter := 5
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				counter--
				if counter == 0 {
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

			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

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

			dv := cdiv1.DataVolume{}
			err := reconciler.createDataVolume(mock, mapper, instance, dv, vmName)

			Expect(err).To(BeNil())
		})
	})

	Describe("manageDataVolumeState step", func() {
		It("should do nothing: ", func() {
			done := map[string]bool{}
			number := len(done)
			conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
			reason := string(v2vv1alpha1.VirtualMachineReady)
			conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
				Status: corev1.ConditionTrue,
				Type:   v2vv1alpha1.Succeeded,
				Reason: &reason,
			})
			instance.Status.Conditions = conditions

			_, err := reconciler.manageDataVolumeState(instance, done, number)

			Expect(err).To(BeNil())
		})

		It("should fail to update status condition: ", func() {
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				return fmt.Errorf("Not modified")
			}
			done := map[string]bool{"test": true, "test2": true}
			number := len(done)

			_, err := reconciler.manageDataVolumeState(instance, done, number)

			Expect(err).To(Not(BeNil()))
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

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

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

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

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

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

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

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

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

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(Not(BeNil()))
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

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
		})

		It("import disk should succeed even if DV failed : ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *cdiv1.DataVolume:
					obj.(*cdiv1.DataVolume).Status = cdiv1.DataVolumeStatus{
						Phase: cdiv1.Failed,
					}
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{}
					obj.(*v2vv1alpha1.VirtualMachineImport).Annotations = map[string]string{sourceVMInitialState: string(provider.VMStatusDown)}
				}
				return nil
			}

			err := reconciler.importDisks(mock, instance, mockMap, vmName)

			Expect(err).To(BeNil())
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.Valid,
					})
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.MappingRulesVerified,
					})
					obj.(*v2vv1alpha1.VirtualMachineImport).Status.Conditions = conditions
					name := "test"
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec.TargetVMName = &name
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					obj.(*v2vv1alpha1.VirtualMachineImport).Annotations = map[string]string{"vmimport.v2v.kubevirt.io/progress": "10"}
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
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
				case *v2vv1alpha1.VirtualMachineImport:
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.Valid,
					})
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.MappingRulesVerified,
					})
					obj.(*v2vv1alpha1.VirtualMachineImport).Status.Conditions = conditions
					name := "test"
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec.TargetVMName = &name
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
				case *v2vv1alpha1.VirtualMachineImport:
					vmImport := obj.(*v2vv1alpha1.VirtualMachineImport)
					vmImport.Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.Valid,
					})
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.MappingRulesVerified,
					})
					vmImport.Status.Conditions = conditions
					vmImport.Status.DataVolumes = []v2vv1alpha1.DataVolumeItem{}
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
			statusPatch = func(ctx context.Context, obj runtime.Object, patch client.Patch) error {
				switch obj.(type) {
				case *kubevirtv1.VirtualMachine:
					return fmt.Errorf("Not modified")
				}

				return nil
			}

			result, err := reconciler.Reconcile(request)

			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("ImportScheduled"))
			event = <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("ImportInProgress"))
			event = <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("DVCreationFailed"))

			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should fail to start vm: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1alpha1.VirtualMachineImport:
					conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
					reason := string(v2vv1alpha1.VirtualMachineReady)
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Reason: &reason,
						Type:   v2vv1alpha1.Succeeded,
					})
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.Valid,
					})
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Type:   v2vv1alpha1.MappingRulesVerified,
					})
					obj.(*v2vv1alpha1.VirtualMachineImport).Status.Conditions = conditions
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					start := true
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec.StartVM = &start
					name := "test"
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec.TargetVMName = &name
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
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should exit early if vm condition is succeed: ", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch obj.(type) {
				case *v2vv1alpha1.VirtualMachineImport:
					conditions := []v2vv1alpha1.VirtualMachineImportCondition{}
					reason := string(v2vv1alpha1.VirtualMachineRunning)
					conditions = append(conditions, v2vv1alpha1.VirtualMachineImportCondition{
						Status: corev1.ConditionTrue,
						Reason: &reason,
						Type:   v2vv1alpha1.Succeeded,
					})
					obj.(*v2vv1alpha1.VirtualMachineImport).Status.Conditions = conditions
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec = v2vv1alpha1.VirtualMachineImportSpec{
						Source: v2vv1alpha1.VirtualMachineImportSourceSpec{
							Ovirt: &v2vv1alpha1.VirtualMachineImportOvirtSourceSpec{},
						},
					}
					start := true
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec.StartVM = &start
					name := "test"
					obj.(*v2vv1alpha1.VirtualMachineImport).Spec.TargetVMName = &name
				case *corev1.Secret:
					obj.(*corev1.Secret).Data = map[string][]byte{"ovirt": getSecret()}
				}
				return nil
			}

			result, err := reconciler.Reconcile(request)

			Expect(err).To(BeNil())
			Expect(result).To(Equal(reconcile.Result{}))
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
		table.Entry("Two disks done progress", map[string]float64{"1": 100, "2": 100}, "85"),
		table.Entry("Two disks half done", map[string]float64{"1": 50, "2": 50}, "47"),
		table.Entry("Two disks one done", map[string]float64{"1": 50, "2": 100}, "66"),
		table.Entry("Done progress", map[string]float64{"1": 100}, "85"),
	)
})

func NewReconciler(client client.Client, finder mappings.ResourceFinder, scheme *runtime.Scheme, ownerreferencesmgr ownerreferences.OwnerReferenceManager, factory pclient.Factory, kvConfigProvider config.KubeVirtConfigProvider, recorder record.EventRecorder, controller controller.Controller) *ReconcileVirtualMachineImport {
	return &ReconcileVirtualMachineImport{
		client:                 client,
		resourceMappingsFinder: finder,
		scheme:                 scheme,
		ownerreferencesmgr:     ownerreferencesmgr,
		factory:                factory,
		kvConfigProvider:       kvConfigProvider,
		recorder:               recorder,
		controller:             controller,
	}
}

type mockClient struct{}

type mockFinder struct{}

type mockProvider struct{}

type mockMapper struct{}

type mockFactory struct{}

type mockController struct{}

type mockKubeVirtConfigProvider struct{}

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
func (m *mockFinder) GetResourceMapping(namespacedName types.NamespacedName) (*v2vv1alpha1.ResourceMapping, error) {
	return getResourceMapping(namespacedName)
}

// Init implements Provider.Init
func (p *mockProvider) Init(secret *corev1.Secret, instance *v2vv1alpha1.VirtualMachineImport) error {
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
func (p *mockProvider) LoadVM(spec v2vv1alpha1.VirtualMachineImportSourceSpec) error {
	return loadVM(spec)
}

// PrepareResourceMapping implements Provider.PrepareResourceMapping
func (p *mockProvider) PrepareResourceMapping(*v2vv1alpha1.ResourceMappingSpec, v2vv1alpha1.VirtualMachineImportSourceSpec) {
}

// LoadVM implements Provider.LoadVM
func (p *mockProvider) Validate() ([]v2vv1alpha1.VirtualMachineImportCondition, error) {
	return validate()
}

// StopVM implements Provider.StopVM
func (p *mockProvider) StopVM(cr *v2vv1alpha1.VirtualMachineImport, client rclient.Client) error {
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
func (p *mockProvider) CleanUp(failure bool, cr *v2vv1alpha1.VirtualMachineImport, client rclient.Client) error {
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
func (m *mockMapper) MapDataVolumes(targetVMName *string) (map[string]cdiv1.DataVolume, error) {
	return map[string]cdiv1.DataVolume{"123": {}}, nil
}

// MapDisks implements Mapper.MapDataVolumes
func (m *mockMapper) MapDisk(vmSpec *kubevirtv1.VirtualMachine, dv cdiv1.DataVolume) {
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

func (c *mockKubeVirtConfigProvider) GetConfig() (config.KubeVirtConfig, error) {
	return getConfig(), nil
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
