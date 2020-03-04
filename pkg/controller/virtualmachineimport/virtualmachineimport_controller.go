package virtualmachineimport

import (
	"context"
	"fmt"

	v2vv1alpha1 "github.com/machacekondra/vm-import-operator/pkg/apis/v2v/v1alpha1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	cdiv1 "github.com/pkliczewski/containerized-data-importer/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_virtualmachineimport")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new VirtualMachineImport Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileVirtualMachineImport{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("virtualmachineimport-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VirtualMachineImport
	err = c.Watch(&source.Kind{Type: &v2vv1alpha1.VirtualMachineImport{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner VirtualMachineImport
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v2vv1alpha1.VirtualMachineImport{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileVirtualMachineImport implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileVirtualMachineImport{}

// ReconcileVirtualMachineImport reconciles a VirtualMachineImport object
type ReconcileVirtualMachineImport struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a VirtualMachineImport object and makes changes based on the state read
// and what is in the VirtualMachineImport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVirtualMachineImport) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling VirtualMachineImport")

	// Fetch the VirtualMachineImport instance
	instance := &v2vv1alpha1.VirtualMachineImport{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Connect to oVirt:
	ovirtSecret, err := r.fetchOvirtSecret(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	ovirtConnection, err := createOvirtConnection(ovirtSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Fetch oVirt VM:
	vm, err := fetchOvirtVM(ovirtConnection, &instance.Spec.Source.Ovirt.VM)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create kubevirt datavolume from oVirt VM disks:
	dvs := newDVForCR(vm, ovirtSecret)
	/*
		for _, dv := range dvs {
			err := r.client.Create(context.TODO(), dv)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	*/

	// Create kubevirt VM from oVirt VM:
	vmSpec := newVMForCR(vm, dvs, ovirtConnection)
	err = r.client.Create(context.TODO(), vmSpec)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set VirtualMachineImport instance as the owner and controller
	/*
		if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if this Pod already exists
		found := &corev1.Pod{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			err = r.client.Create(context.TODO(), pod)
			if err != nil {
				return reconcile.Result{}, err
			}

			// Pod created successfully - don't requeue
			return reconcile.Result{}, nil
		} else if err != nil {
			return reconcile.Result{}, err
		}

		// Pod already exists - don't requeue
		reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	*/
	return reconcile.Result{}, nil
}

func fetchOvirtVM(con *ovirtsdk.Connection, vmSpec *v2vv1alpha1.VirtualMachineImportOvirtSourceVMSpec) (*ovirtsdk.Vm, error) {
	// Id of the VM specified:
	if vmSpec.ID != "" {
		response, err := con.SystemService().VmsService().VmService(vmSpec.ID).Get().Send()
		if err != nil {
			return nil, err
		}
		vm, _ := response.Vm()
		return vm, nil
	}
	// Cluster/name of the VM specified:
	response, err := con.SystemService().VmsService().List().Search(fmt.Sprintf("name=%v and cluster=%v", vmSpec.Name, vmSpec.Cluster)).Send()
	if err != nil {
		return nil, err
	}
	vms, _ := response.Vms()
	if len(vms.Slice()) > 0 {
		return vms.Slice()[0], nil
	}
	return nil, fmt.Errorf("Virtual machine %v not found", vmSpec.Name)
}

func (r *ReconcileVirtualMachineImport) fetchOvirtSecret(vmImport *v2vv1alpha1.VirtualMachineImport) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: vmImport.Spec.Source.Ovirt.SecretName, Namespace: vmImport.Namespace}, secret)
	return secret, err
}

func createOvirtConnection(secret *corev1.Secret) (*ovirtsdk.Connection, error) {
	// TODO: CA cert
	return ovirtsdk.NewConnectionBuilder().
		URL(string(secret.Data["apiUrl"])).
		Username(string(secret.Data["username"])).
		Password(string(secret.Data["password"])).
		Insecure(true).
		Build()
}

// newDVForCR returns a VM specification of the fetched oVirt VM
func newDVForCR(vm *ovirtsdk.Vm, secret *corev1.Secret) []cdiv1.DataVolume {
	quantity, _ := resource.ParseQuantity("1Gi")
	diskAttachments, _ := vm.DiskAttachments()
	dvs := make([]cdiv1.DataVolume, len(diskAttachments.Slice()))
	for _, diskAttachment := range diskAttachments.Slice() {
		disk, _ := diskAttachment.Disk()
		dvs = append(dvs,
			cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      disk.MustId(), // FIXME:
					Namespace: "default",     // FIXME:
					Labels: map[string]string{ // FIXME:
						"origin": "oVirt",
					},
				},
				Spec: cdiv1.DataVolumeSpec{
					Source: cdiv1.DataVolumeSource{
						/*
							Imageio: &cdiv1.DataVolumeSourceImageIO{
								URL:           secret.Data["apiUrl"],
								DiskID:        vm.MustDiskAttachments().Slice()[0].MustDisk().MustId(),
								SecretRef:     "",
								CertConfigMap: "",
							},
						*/
					},
					PVC: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: quantity,
							},
						},
					},
				},
			},
		)
	}
	return dvs
}

// newVMForCR returns a VM specification of the fetched oVirt VM
func newVMForCR(vm *ovirtsdk.Vm, dvs []cdiv1.DataVolume, con *ovirtsdk.Connection) *kubevirtv1.VirtualMachine {
	labels := map[string]string{
		"origin": "oVirt",
	}
	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vm.MustName(), // FIXME:
			Namespace: "default",     // FIXME:
			Labels:    labels,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						CPU: &kubevirtv1.CPU{
							Cores: uint32(vm.MustCpu().MustTopology().MustCores()),
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								kubevirtv1.Disk{
									Name: "disk1",
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
							// Memory:  &kubevirtv1.Memory{},
							// Machine:   kubevirtv1.Machine{},
							// Firmware:  &kubevirtv1.Firmware{},
							// Clock:     &kubevirtv1.Clock{},
							// Features:  &kubevirtv1.Features{},
							// Chassis:   &kubevirtv1.Chassis{},
							// IOThreadsPolicy: &kubevirtv1.IOThreadsPolicy{},
						},
					},
					Volumes: []kubevirtv1.Volume{
						kubevirtv1.Volume{
							Name: "dv", // TODO:
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: dvs[0].Name,
								},
							},
						},
					},
				},
			},
		},
	}
}
