package virtualmachineimport

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	ovirtclient "github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/client"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"gopkg.in/yaml.v2"
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

const (
	ovirtSecret    = "ovirt-key"
	ovirtConfigmap = "ovirt-ca"
	cdiAPIVersion  = "cdi.kubevirt.io/v1alpha1"
	ovirtLabel     = "oVirt"
	ovirtSecretKey = "ovirt"
)

var (
	log    = logf.Log.WithName("controller_virtualmachineimport")
	labels = map[string]string{
		"origin": ovirtLabel,
	}
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
	ovirtSecretObj, err := r.fetchOvirtSecret(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	ovirtSecretDataMap := make(map[string]string)
	err = yaml.Unmarshal(ovirtSecretObj.Data[ovirtSecretKey], &ovirtSecretDataMap)
	if err != nil {
		return reconcile.Result{}, err
	}

	if _, ok := ovirtSecretDataMap["caCert"]; !ok {
		return reconcile.Result{}, fmt.Errorf("oVirt secret must contain caCert attribute")
	}
	if len(ovirtSecretDataMap["caCert"]) == 0 {
		return reconcile.Result{}, fmt.Errorf("oVirt secret caCert cannot be empty")
	}
	caCert, err := base64.StdEncoding.DecodeString(ovirtSecretDataMap["caCert"])
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("Failed to decode oVirt secret caCert to base64 format")
	}
	ovirt, err := ovirtclient.NewRichOvirtClient(
		&ovirtclient.ConnectionSettings{
			URL:      ovirtSecretDataMap["apiUrl"],
			Username: ovirtSecretDataMap["username"],
			Password: ovirtSecretDataMap["password"],
			CACert:   caCert,
		},
	)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer ovirt.Close()

	// Fetch oVirt VM:
	sourceVMID := instance.Spec.Source.Ovirt.VM.ID
	sourceVMName := instance.Spec.Source.Ovirt.VM.Name
	var sourceVMClusterName *string
	var sourceVMClusterID *string
	if instance.Spec.Source.Ovirt.VM.Cluster != nil {
		sourceVMClusterName = instance.Spec.Source.Ovirt.VM.Cluster.Name
		sourceVMClusterID = instance.Spec.Source.Ovirt.VM.Cluster.ID
	}
	vm, err := ovirt.GetVM(sourceVMID, sourceVMName, sourceVMClusterName, sourceVMClusterID)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Stop VM
	vmID, _ := vm.Id()
	err = ovirt.StopVM(vmID)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Define VM spec
	vmSpec := createVMSpec(vm, instance)

	// Set VirtualMachineImport instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, vmSpec, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this VM already exists
	found := &kubevirtv1.VirtualMachine{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: vmSpec.Name, Namespace: vmSpec.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create kubevirt VM from oVirt VM:
		reqLogger.Info("Creating a new VM", "VM.Namespace", vmSpec.Namespace, "VM.Name", vmSpec.Name)
		err = r.client.Create(context.TODO(), vmSpec)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Secret with username/password for the image import:
		secretObj := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: ovirtSecret, Namespace: vmSpec.Namespace}, secretObj)
		if err != nil && errors.IsNotFound(err) {
			dvSecret := createDVSecret(ovirtSecretDataMap, instance)
			err = r.client.Create(context.TODO(), dvSecret)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// CM containing CA for the image import:
		cm := &corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: ovirtConfigmap, Namespace: vmSpec.Namespace}, cm)
		if err != nil && errors.IsNotFound(err) {
			dvCm := createDVConfigmap(ovirtSecretDataMap, instance)
			err = r.client.Create(context.TODO(), dvCm)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Import disks:
		dvs := createDVmap(vm, ovirtSecretDataMap, instance.Namespace)
		for _, dv := range dvs {
			_, err := r.kubeClient.CdiClient().CdiV1alpha1().DataVolumes(instance.Namespace).Create(&dv)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Update VM spec with disks:
		vmDef := &kubevirtv1.VirtualMachine{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: vmSpec.Namespace, Name: vmSpec.Name}, vmDef)
		if err != nil {
			return reconcile.Result{}, err
		}
		updateVM(vmDef, dvs, vm)
		err = r.client.Update(context.TODO(), vmDef)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Rename VM
		/*
			vmName, _ := vm.Name()
			err = ovirt.RenameVM(vmID, fmt.Sprintf("%s_exported", vmName))
			if err != nil {
				return reconcile.Result{}, err
			}
		*/

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// VM already exists - don't requeue
	reqLogger.Info("Skip reconcile: VM already exists", "VM.Namespace", found.Namespace, "VM.Name", found.Name)

	return reconcile.Result{}, nil
}

func createDVConfigmap(creds map[string]string, vmImport *v2vv1alpha1.VirtualMachineImport) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ovirtConfigmap,
			Namespace: vmImport.Namespace,
		},
		Data: map[string]string{
			"ca.pem": creds["caCert"],
		},
	}
}

func createDVSecret(creds map[string]string, vmImport *v2vv1alpha1.VirtualMachineImport) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ovirtSecret,
			Namespace: vmImport.Namespace,
		},
		Data: map[string][]byte{
			"accessKeyId": []byte(creds["username"]),
			"secretKey":   []byte(creds["password"]),
		},
	}
}

func createVMSpec(vm *ovirtsdk.Vm, vmImport *v2vv1alpha1.VirtualMachineImport) *kubevirtv1.VirtualMachine {
	cpu := &kubevirtv1.CPU{}
	if cpuDef, available := vm.Cpu(); available {
		if topology, available := cpuDef.Topology(); available {
			if cores, available := topology.Cores(); available {
				cpu.Cores = uint32(cores)
			}
			if sockets, available := topology.Sockets(); available {
				cpu.Sockets = uint32(sockets)
			}
			if threads, available := topology.Threads(); available {
				cpu.Threads = uint32(threads)
			}
		}
	}
	running := false
	name, _ := vm.Name()
	if vmImport.Spec.TargetVMName != nil {
		name = *vmImport.Spec.TargetVMName
	}
	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: vmImport.Namespace,
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
						CPU: cpu,
						// Memory:  &kubevirtv1.Memory{},
						// Machine:   kubevirtv1.Machine{},
						// Firmware:  &kubevirtv1.Firmware{},
						// Clock:     &kubevirtv1.Clock{},
						// Features:  &kubevirtv1.Features{},
						// Chassis:   &kubevirtv1.Chassis{},
						// IOThreadsPolicy: &kubevirtv1.IOThreadsPolicy{},
					},
				},
			},
		},
	}
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

// newDVForCR returns the data-volume specifications for the target VM based on oVirt VM
func createDVmap(vm *ovirtsdk.Vm, creds map[string]string, namespace string) map[string]cdiv1.DataVolume {
	diskAttachments, _ := vm.DiskAttachments()
	dvs := make(map[string]cdiv1.DataVolume, len(diskAttachments.Slice()))
	for _, diskAttachment := range diskAttachments.Slice() {
		attachID, _ := diskAttachment.Id()
		disk, _ := diskAttachment.Disk()
		diskID, _ := disk.Id()
		quantity, _ := resource.ParseQuantity(strconv.FormatInt(disk.MustProvisionedSize(), 10))
		dvs[attachID] = cdiv1.DataVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: cdiAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      attachID,
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: cdiv1.DataVolumeSource{
					Imageio: &cdiv1.DataVolumeSourceImageIO{
						URL:           creds["apiUrl"],
						DiskID:        diskID,
						SecretRef:     ovirtSecret,
						CertConfigMap: ovirtConfigmap,
					},
				},
				// TODO: Should be done according to mappings
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
func updateVM(vmspec *kubevirtv1.VirtualMachine, dvs map[string]cdiv1.DataVolume, vm *ovirtsdk.Vm) {
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
		diskAttachment := getDiskAttachmentByID(id, vm.MustDiskAttachments())
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
	vmspec.Spec = kubevirtv1.VirtualMachineSpec{
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
	}
}
