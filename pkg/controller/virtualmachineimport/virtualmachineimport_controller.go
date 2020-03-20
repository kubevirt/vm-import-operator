package virtualmachineimport

import (
	"context"
	"fmt"
	"strconv"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	ovirtsdk "github.com/ovirt/go-ovirt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_virtualmachineimport")

const (
	cdiAPIVersion = "cdi.kubevirt.io/v1alpha1"
	ovirtLabel    = "oVirt"
)

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
	kubeClient, err := kubecli.GetKubevirtClientFromRESTConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "Unable to get KubeVirt client")
		panic("Controller cannot operate without KubeVirt")
	}
	return &ReconcileVirtualMachineImport{client: mgr.GetClient(), scheme: mgr.GetScheme(), kubeClient: kubeClient}
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
	err = c.Watch(&source.Kind{Type: &kubevirtv1.VirtualMachine{}}, &handler.EnqueueRequestForOwner{
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
	client     client.Client
	scheme     *runtime.Scheme
	kubeClient kubecli.KubevirtClient
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

	diskAttachmentsLink, _ := vm.DiskAttachments()
	diskAttachments, _ := ovirtConnection.FollowLink(diskAttachmentsLink)
	dvs := createDVmap(diskAttachments.(*ovirtsdk.DiskAttachmentSlice), ovirtSecret)
	vmSpec := newVMForCR(vm, dvs, diskAttachments.(*ovirtsdk.DiskAttachmentSlice))

	// Set VirtualMachineImport instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, vmSpec, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this VM already exists
	found := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: vmSpec.Name, Namespace: vmSpec.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new VM", "VM.Namespace", vmSpec.Namespace, "VM.Name", vmSpec.Name)

		// Create kubevirt datavolume from oVirt VM disks:
		for _, dv := range dvs {
			_, err := r.kubeClient.CdiClient().CdiV1alpha1().DataVolumes(instance.Namespace).Create(&dv)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Create kubevirt VM from oVirt VM:
		// TODO: create vm at early stage and patch it later with additional data
		err = r.client.Create(context.TODO(), vmSpec)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// VM already exists - don't requeue
	reqLogger.Info("Skip reconcile: VM already exists", "VM.Namespace", found.Namespace, "VM.Name", found.Name)

	return reconcile.Result{}, nil
}

func fetchOvirtVM(con *ovirtsdk.Connection, vmSpec *v2vv1alpha1.VirtualMachineImportOvirtSourceVMSpec) (*ovirtsdk.Vm, error) {
	// Id of the VM specified:
	if vmSpec.ID != nil {
		response, err := con.SystemService().VmsService().VmService(*vmSpec.ID).Get().Send()
		if err != nil {
			return nil, err
		}
		vm, _ := response.Vm()
		return vm, nil
	}
	// Cluster/name of the VM specified:
	response, err := con.SystemService().VmsService().List().Search(fmt.Sprintf("name=%v and cluster=%v", *vmSpec.Name, *vmSpec.Cluster.Name)).Send()
	if err != nil {
		return nil, err
	}
	vms, _ := response.Vms()
	if len(vms.Slice()) > 0 {
		return vms.Slice()[0], nil
	}
	return nil, fmt.Errorf("Virtual machine %v not found", *vmSpec.Name)
}

func (r *ReconcileVirtualMachineImport) fetchOvirtSecret(vmImport *v2vv1alpha1.VirtualMachineImport) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretNamespace := vmImport.Namespace
	if vmImport.Spec.ProviderCredentialsSecret.Namespace != nil {
		secretNamespace = *vmImport.Spec.ProviderCredentialsSecret.Namespace
	}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: vmImport.Spec.ProviderCredentialsSecret.Name, Namespace: secretNamespace}, secret)
	return secret, err
}

func createOvirtConnection(secret *corev1.Secret) (*ovirtsdk.Connection, error) {
	// TODO: CA cert
	insecure, _ := strconv.ParseBool(string(secret.Data["insecure"]))
	return ovirtsdk.NewConnectionBuilder().
		URL(string(secret.Data["apiUrl"])).
		Username(string(secret.Data["username"])).
		Password(string(secret.Data["password"])).
		Insecure(insecure).
		Build()
}

// newDVForCR returns the data-volume specifications for the target VM based on oVirt VM
func createDVmap(diskAttachments *ovirtsdk.DiskAttachmentSlice, secret *corev1.Secret) map[string]cdiv1.DataVolume {
	dvs := make(map[string]cdiv1.DataVolume, len(diskAttachments.Slice()))
	for _, diskAttachment := range diskAttachments.Slice() {
		disk, _ := diskAttachment.Disk()
		//quantity, _ := resource.ParseQuantity(strconv.FormatInt(disk.MustProvisionedSize(), 10))
		quantity, _ := resource.ParseQuantity("1Gi")
		dvs[diskAttachment.MustId()] = cdiv1.DataVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: cdiAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      diskAttachment.MustId(), // FIXME:
				Namespace: "default",               // FIXME:
				Labels: map[string]string{ // FIXME:
					"origin": ovirtLabel,
				},
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: cdiv1.DataVolumeSource{
					Imageio: &cdiv1.DataVolumeSourceImageIO{
						URL:           string(secret.Data["apiUrl"]),
						DiskID:        disk.MustId(),
						SecretRef:     "ovirt-key", // FIXME, should be created dynamically
						CertConfigMap: "ovirt-ca",  // FIXME, should be created dynamically
					},
				},
				// TODO: Would be great to add storageClassName which we should get from the mapping.
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
		}
	}
	return dvs
}

func getDiskAttachmentByID(id string, diskAttachments *ovirtsdk.DiskAttachmentSlice) *ovirtsdk.DiskAttachment {
	for _, diskAttachment := range diskAttachments.Slice() {
		if diskAttachment.MustId() == id {
			return diskAttachment
		}
	}
	return nil
}

// newVMForCR returns a VM specification of the fetched oVirt VM
func newVMForCR(vm *ovirtsdk.Vm, dvs map[string]cdiv1.DataVolume, diskAttachments *ovirtsdk.DiskAttachmentSlice) *kubevirtv1.VirtualMachine {
	// Labels definition:
	labels := map[string]string{
		"origin": ovirtLabel,
	}
	// Volumes definition:
	volumes := make([]kubevirtv1.Volume, len(dvs))
	i := 0
	for _, dv := range dvs {
		volumes[i] = kubevirtv1.Volume{
			Name: fmt.Sprintf("dv-%v", i),
			VolumeSource: kubevirtv1.VolumeSource{
				DataVolume: &kubevirtv1.DataVolumeSource{
					Name: dv.Name,
				},
			},
		}
		i++
	}
	// Disks definition:
	i = 0
	disks := make([]kubevirtv1.Disk, len(dvs))
	for id := range dvs {
		diskAttachment := getDiskAttachmentByID(id, diskAttachments)
		disks[i] = kubevirtv1.Disk{
			Name: fmt.Sprintf("dv-%v", i),
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: string(diskAttachment.MustInterface()),
				},
			},
		}
		if diskAttachment.MustBootable() {
			bootOrder := uint(1)
			disks[i].BootOrder = &bootOrder
		}

		i++
	}
	running := false
	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vm.MustName(), // FIXME:
			Namespace: "default",     // FIXME:
			Labels:    labels,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Running: &running,
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
							Disks: disks,
							// Memory:  &kubevirtv1.Memory{},
							// Machine:   kubevirtv1.Machine{},
							// Firmware:  &kubevirtv1.Firmware{},
							// Clock:     &kubevirtv1.Clock{},
							// Features:  &kubevirtv1.Features{},
							// Chassis:   &kubevirtv1.Chassis{},
							// IOThreadsPolicy: &kubevirtv1.IOThreadsPolicy{},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}
