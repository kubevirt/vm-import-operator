package controller

import (
	"context"
	generrors "errors"
	"fmt"
	"os"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	conditions "github.com/openshift/custom-resource-status/conditions/v1"

	vmimportv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	resources "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	realClient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	Version   = "v0.0.2"
	Namespace = "kubevirt-hyperconverged"
)

type args struct {
	config     *vmimportv1alpha1.VMImportConfig
	client     client.Client
	reconciler *ReconcileVMImportConfig
}

var (
	envVars = map[string]string{
		"OPERATOR_VERSION":         Version,
		"DEPLOY_CLUSTER_RESOURCES": "true",
		"CONTROLLER_IMAGE":         "kubevirt/vm-import-controller",
		"PULL_POLICY":              "Always",
	}
)

func init() {
	vmimportv1alpha1.AddToScheme(scheme.Scheme)
	extv1beta1.AddToScheme(scheme.Scheme)
}

type modifyResource func(toModify runtime.Object) (runtime.Object, runtime.Object, error)
type isModifySubject func(resource runtime.Object) bool
type isUpgraded func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool

type createUnusedObject func() (runtime.Object, error)

var _ = Describe("Controller", func() {
	Describe("controller runtime bootstrap test", func() {
		Context("Create manager and controller", func() {
			BeforeEach(func() {
				for k, v := range envVars {
					os.Setenv(k, v)
				}
			})

			AfterEach(func() {
				for k := range envVars {
					os.Unsetenv(k)
				}
			})

			It("should succeed", func() {
				mgr, err := manager.New(cfg, manager.Options{})
				Expect(err).ToNot(HaveOccurred())

				err = vmimportv1alpha1.AddToScheme(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = extv1beta1.AddToScheme(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = Add(mgr)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	DescribeTable("check can create types", func(obj runtime.Object) {
		client := createClient(obj)

		_, err := getObject(client, obj)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("VMImportConfig type", createConfig("vm-import-config", "I am unique")),
		Entry("CRD type", &extv1beta1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "crd"}}),
	)

	Describe("Deploying Config", func() {
		Context("Config lifecycle", func() {
			It("should get deployed", func() {
				args := createArgs()
				doReconcile(args)
				setDeploymentsReady(args)

				Expect(args.config.Status.OperatorVersion).Should(Equal(Version))
				Expect(args.config.Status.TargetVersion).Should(Equal(Version))
				Expect(args.config.Status.ObservedVersion).Should(Equal(Version))

				Expect(args.config.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.config.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())
			})

			It("should create all resources", func() {
				args := createArgs()
				doReconcile(args)

				resources, err := args.reconciler.getAllResources(args.config)
				Expect(err).ToNot(HaveOccurred())

				for _, r := range resources {
					_, err := getObject(args.client, r)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("can become become ready, un-ready, and ready again", func() {
				var deployment *appsv1.Deployment

				args := createArgs()
				doReconcile(args)

				resources, err := args.reconciler.getAllResources(args.config)
				Expect(err).ToNot(HaveOccurred())

				for _, r := range resources {
					d, ok := r.(*appsv1.Deployment)
					if !ok {
						continue
					}

					dd, err := getDeployment(args.client, d)
					Expect(err).ToNot(HaveOccurred())

					dd.Status.Replicas = *dd.Spec.Replicas
					dd.Status.ReadyReplicas = dd.Status.Replicas

					err = args.client.Update(context.TODO(), dd)
					Expect(err).ToNot(HaveOccurred())
				}

				doReconcile(args)

				Expect(args.config.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.config.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

				for _, r := range resources {
					var ok bool
					deployment, ok = r.(*appsv1.Deployment)
					if ok {
						break
					}
				}

				deployment, err = getDeployment(args.client, deployment)
				Expect(err).ToNot(HaveOccurred())
				deployment.Status.ReadyReplicas = 0
				err = args.client.Update(context.TODO(), deployment)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.config.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.config.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				// Application should be degraded due to missing deployment pods (set to 0)
				Expect(conditions.IsStatusConditionTrue(args.config.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

				deployment, err = getDeployment(args.client, deployment)
				Expect(err).ToNot(HaveOccurred())
				deployment.Status.ReadyReplicas = deployment.Status.Replicas
				err = args.client.Update(context.TODO(), deployment)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.config.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.config.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())
			})

			It("should succeed when we delete VMImportConfig", func() {
				// create random pod that should get deleted
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod",
						Namespace: "default",
						Labels: map[string]string{
							"v2v.kubevirt.io": "",
						},
					},
				}

				args := createArgs()
				doReconcile(args)

				err := args.client.Create(context.TODO(), pod)
				Expect(err).ToNot(HaveOccurred())

				args.config.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				err = args.client.Update(context.TODO(), args.config)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeleted))

				_, err = getObject(args.client, pod)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	Describe("Upgrading VMImport", func() {
		DescribeTable("check detects upgrade correctly", func(prevVersion, newVersion string, shouldUpgrade, shouldError bool) {
			//verify on int version is set
			args := createFromArgs(Namespace, newVersion)
			doReconcile(args)
			setDeploymentsReady(args)

			Expect(args.config.Status.ObservedVersion).Should(Equal(newVersion))
			Expect(args.config.Status.OperatorVersion).Should(Equal(newVersion))
			Expect(args.config.Status.TargetVersion).Should(Equal(newVersion))
			Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

			//Modify CRD to be of previousVersion
			err := args.reconciler.crSetVersion(args.config, prevVersion)
			Expect(err).ToNot(HaveOccurred())

			if shouldError {
				doReconcileError(args)
				return
			}

			setDeploymentsDegraded(args)
			doReconcile(args)

			if shouldUpgrade {
				//verify upgraded has started
				Expect(args.config.Status.OperatorVersion).Should(Equal(newVersion))
				Expect(args.config.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.TargetVersion).Should(Equal(newVersion))
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseUpgrading))
			} else {
				//verify upgraded hasn't started
				Expect(args.config.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))
			}

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			//now should be upgraded
			if shouldUpgrade {
				//verify versions were updated
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))
				Expect(args.config.Status.OperatorVersion).Should(Equal(newVersion))
				Expect(args.config.Status.TargetVersion).Should(Equal(newVersion))
				Expect(args.config.Status.ObservedVersion).Should(Equal(newVersion))
			} else {
				//verify versions remained unchaged
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))
				Expect(args.config.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.ObservedVersion).Should(Equal(prevVersion))
			}
		},
			Entry("increasing semver ", "v0.0.1", "v0.0.2", true, false),
			Entry("decreasing semver", "v0.0.2", "v0.0.1", false, true),
			Entry("identical semver", "v0.0.2", "v0.0.2", false, false),
			Entry("invalid semver", "latest", "v0.0.1", true, false),
		)

		Describe("VMImportConfig CR deletion during upgrade", func() {
			It("should delete CR if it is marked for deletion and not begin upgrade flow", func() {
				var args *args
				newVersion := "v0.0.2"
				prevVersion := "v0.0.1"

				args = createFromArgs(Namespace, newVersion)
				doReconcile(args)

				//set deployment to ready
				isReady := setDeploymentsReady(args)
				Expect(isReady).Should(Equal(true))

				//verify on int version is set
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

				//Modify CRD to be of previousVersion
				args.reconciler.crSetVersion(args.config, prevVersion)
				//mark CR for deltetion
				args.config.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
				err := args.client.Update(context.TODO(), args.config)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				//verify the version cr is deleted and upgrade hasn't started
				Expect(args.config.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeleted))
			})

			It("should delete CR if it is marked for deletion during upgrade flow", func() {
				var args *args
				newVersion := "v0.0.2"
				prevVersion := "v0.0.1"

				args = createFromArgs(Namespace, newVersion)
				doReconcile(args)
				setDeploymentsReady(args)

				//verify on int version is set
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

				//Modify CRD to be of previousVersion
				args.reconciler.crSetVersion(args.config, prevVersion)
				err := args.client.Update(context.TODO(), args.config)
				Expect(err).ToNot(HaveOccurred())
				setDeploymentsDegraded(args)

				//begin upgrade
				doReconcile(args)

				//mark CR for deltetion
				args.config.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
				err = args.client.Update(context.TODO(), args.config)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				//set deployment to ready
				isReady := setDeploymentsReady(args)
				Expect(isReady).Should(Equal(false))

				doReconcile(args)
				//verify the version cr is marked as deleted
				Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeleted))
			})
		})
	})

	DescribeTable("Updates objects on upgrade", func(modify modifyResource, tomodify isModifySubject, upgraded isUpgraded) {
		var args *args
		newVersion := "v0.0.2"
		prevVersion := "v0.0.1"

		args = createFromArgs(Namespace, newVersion)
		doReconcile(args)
		setDeploymentsReady(args)

		//verify on int version is set
		Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

		//Modify CRD to be of previousVersion
		args.reconciler.crSetVersion(args.config, prevVersion)
		err := args.client.Update(context.TODO(), args.config)
		Expect(err).ToNot(HaveOccurred())

		setDeploymentsDegraded(args)

		//find the resource to modify
		oOriginal, oModified, err := getModifiedResource(args.reconciler, args.config, modify, tomodify)
		Expect(err).ToNot(HaveOccurred())

		//update object via client, with curObject
		err = args.client.Update(context.TODO(), oModified)
		Expect(err).ToNot(HaveOccurred())

		//verify object is modified
		storedObj, err := getObject(args.client, oModified)
		Expect(err).ToNot(HaveOccurred())

		Expect(reflect.DeepEqual(storedObj, oModified)).Should(Equal(true))

		doReconcile(args)

		//verify upgraded has started
		Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseUpgrading))

		//change deployment to ready
		isReady := setDeploymentsReady(args)
		Expect(isReady).Should(Equal(true))

		doReconcile(args)
		Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

		//verify that stored object equals to object in getResources
		storedObj, err = getObject(args.client, oModified)
		Expect(err).ToNot(HaveOccurred())

		Expect(upgraded(storedObj, oOriginal)).Should(Equal(true))
	},
		Entry("verify - deployment updated on upgrade - deployment spec changed - modify container",
			func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
				deploymentOrig, ok := toModify.(*appsv1.Deployment)
				if !ok {
					return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
				}
				deployment := deploymentOrig.DeepCopy()

				containers := deployment.Spec.Template.Spec.Containers
				containers[0].Env = []corev1.EnvVar{
					{
						Name:  "FAKE_ENVVAR",
						Value: fmt.Sprintf("%s/%s:%s", "fake_repo", "importerImage", "tag"),
					},
				}

				return toModify, deployment, nil
			},
			func(resource runtime.Object) bool { //find resource for test
				deployment, ok := resource.(*appsv1.Deployment)
				if !ok {
					return false
				}
				if deployment.Name == "vm-import-deployment" {
					return true
				}
				return false
			},
			func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
				postDep, ok := postUpgradeObj.(*appsv1.Deployment)
				if !ok {
					return false
				}

				desiredDep, ok := deisredObj.(*appsv1.Deployment)
				if !ok {
					return false
				}

				for key, envVar := range desiredDep.Spec.Template.Spec.Containers[0].Env {
					if postDep.Spec.Template.Spec.Containers[0].Env[key].Name != envVar.Name {
						return false
					}
				}

				if len(desiredDep.Spec.Template.Spec.Containers[0].Env) != len(postDep.Spec.Template.Spec.Containers[0].Env) {
					return false
				}

				return true
			}),
		Entry("verify - deployment updated on upgrade - deployment spec changed - add new container",
			func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
				deploymentOrig, ok := toModify.(*appsv1.Deployment)
				if !ok {
					return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
				}
				deployment := deploymentOrig.DeepCopy()

				containers := deployment.Spec.Template.Spec.Containers
				container := corev1.Container{
					Name:            "FAKE_CONTAINER",
					Image:           fmt.Sprintf("%s/%s:%s", "fake-repo", "fake-image", "fake-tag"),
					ImagePullPolicy: "FakePullPolicy",
					Args:            []string{"-v=10"},
				}
				containers = append(containers, container)

				return toModify, deployment, nil
			},
			func(resource runtime.Object) bool { //find resource for test
				deployment, ok := resource.(*appsv1.Deployment)
				if !ok {
					return false
				}
				if deployment.Name == "vm-import-deployment" {
					return true
				}
				return false
			},
			func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
				postDep, ok := postUpgradeObj.(*appsv1.Deployment)
				if !ok {
					return false
				}

				desiredDep, ok := deisredObj.(*appsv1.Deployment)
				if !ok {
					return false
				}

				for key, container := range desiredDep.Spec.Template.Spec.Containers {
					if postDep.Spec.Template.Spec.Containers[key].Name != container.Name {
						return false
					}
				}

				if len(desiredDep.Spec.Template.Spec.Containers) > len(postDep.Spec.Template.Spec.Containers) {
					return false
				}

				return true
			}),
		Entry("verify - deployment updated on upgrade - deployment spec changed - remove existing container",
			func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
				deploymentOrig, ok := toModify.(*appsv1.Deployment)
				if !ok {
					return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
				}
				deployment := deploymentOrig.DeepCopy()

				deployment.Spec.Template.Spec.Containers = nil

				return toModify, deployment, nil
			},
			func(resource runtime.Object) bool { //find resource for test
				deployment, ok := resource.(*appsv1.Deployment)
				if !ok {
					return false
				}
				if deployment.Name == "vm-import-deployment" {
					return true
				}
				return false
			},
			func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
				postDep, ok := postUpgradeObj.(*appsv1.Deployment)
				if !ok {
					return false
				}

				desiredDep, ok := deisredObj.(*appsv1.Deployment)
				if !ok {
					return false
				}

				return (len(postDep.Spec.Template.Spec.Containers) == len(desiredDep.Spec.Template.Spec.Containers))
			}),
	)

	DescribeTable("Removes unused objects on upgrade", func(createObj createUnusedObject) {
		var args *args
		newVersion := "v0.0.2"
		prevVersion := "v0.0.1"

		args = createFromArgs(Namespace, newVersion)
		doReconcile(args)

		setDeploymentsReady(args)

		//verify on int version is set
		Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

		//Modify CRD to be of previousVersion
		args.reconciler.crSetVersion(args.config, prevVersion)
		err := args.client.Update(context.TODO(), args.config)
		Expect(err).ToNot(HaveOccurred())

		setDeploymentsDegraded(args)
		unusedObj, err := createObj()
		Expect(err).ToNot(HaveOccurred())
		unusedMetaObj := unusedObj.(metav1.Object)
		unusedMetaObj.SetLabels(make(map[string]string))
		unusedMetaObj.GetLabels()["operator.v2v.kubevirt.io/createVersion"] = prevVersion
		err = controllerutil.SetControllerReference(args.config, unusedMetaObj, scheme.Scheme)
		Expect(err).ToNot(HaveOccurred())

		//add unused object via client, with curObject
		err = args.client.Create(context.TODO(), unusedObj)
		Expect(err).ToNot(HaveOccurred())

		doReconcile(args)

		//verify upgraded has started
		Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseUpgrading))

		//verify unused exists before upgrade is done
		_, err = getObject(args.client, unusedObj)
		Expect(err).ToNot(HaveOccurred())

		//change deployment to ready
		isReady := setDeploymentsReady(args)
		Expect(isReady).Should(Equal(true))

		doReconcile(args)
		Expect(args.config.Status.Phase).Should(Equal(vmimportv1alpha1.PhaseDeployed))

		//verify that object no longer exists after upgrade
		_, err = getObject(args.client, unusedObj)
		Expect(errors.IsNotFound(err)).Should(Equal(true))
	},
		Entry("verify - unused deployment deleted",
			func() (runtime.Object, error) {
				deployment := resources.CreateControllerDeployment("fake-deployment", Namespace, "fake-vmimport", "Always", int32(1))
				return deployment, nil
			}),

		Entry("verify - unused crd deleted",
			func() (runtime.Object, error) {
				crd := &extv1beta1.CustomResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.k8s.io/v1beta1",
						Kind:       "CustomResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake.vmimportconfigs.v2v.kubevirt.io",
						Labels: map[string]string{
							"operator.v2v.kubevirt.io": "",
						},
					},
					Spec: extv1beta1.CustomResourceDefinitionSpec{
						Group:   "v2v.kubevirt.io",
						Version: "v1alpha1",
						Scope:   "Cluster",

						Versions: []extv1beta1.CustomResourceDefinitionVersion{
							{
								Name:    "v1alpha1",
								Served:  true,
								Storage: true,
							},
						},
						Names: extv1beta1.CustomResourceDefinitionNames{
							Kind:     "FakeVMImportConfig",
							ListKind: "VMImportConfigList",
							Plural:   "fakevmimportconfigs",
							Singular: "fakevmimportconfig",
							Categories: []string{
								"all",
							},
							ShortNames: []string{"fakevmimportconfig", "fakevmimportconfigs"},
						},

						AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
							{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
							{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
						},
					},
				}
				return crd, nil
			}),
	)
})

func setDeploymentsDegraded(args *args) {
	resources, err := args.reconciler.getAllResources(args.config)
	Expect(err).ToNot(HaveOccurred())

	for _, r := range resources {
		d, ok := r.(*appsv1.Deployment)
		if !ok {
			continue
		}

		d, err := getDeployment(args.client, d)
		Expect(err).ToNot(HaveOccurred())
		if d.Spec.Replicas != nil {
			d.Status.Replicas = int32(0)
			d.Status.ReadyReplicas = d.Status.Replicas
			err = args.client.Update(context.TODO(), d)
			Expect(err).ToNot(HaveOccurred())
		}

	}
	doReconcile(args)
}

func doReconcileError(args *args) {
	result, err := args.reconciler.Reconcile(reconcileRequest(args.config.Name))
	Expect(err).To(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	args.config, err = getConfig(args.client, args.config)
	Expect(err).ToNot(HaveOccurred())
}

func createClient(objs ...runtime.Object) realClient.Client {
	return fakeClient.NewFakeClientWithScheme(scheme.Scheme, objs...)
}

func createConfig(name, uid string) *vmimportv1alpha1.VMImportConfig {
	return &vmimportv1alpha1.VMImportConfig{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(uid)}}
}

func getObject(client realClient.Client, obj runtime.Object) (runtime.Object, error) {
	metaObj := obj.(metav1.Object)
	key := realClient.ObjectKey{Namespace: metaObj.GetNamespace(), Name: metaObj.GetName()}

	typ := reflect.ValueOf(obj).Elem().Type()
	result := reflect.New(typ).Interface().(runtime.Object)

	if err := client.Get(context.TODO(), key, result); err != nil {
		return nil, err
	}

	return result, nil
}

func createArgs() *args {
	return createFromArgs(Namespace, Version)
}

func createFromArgs(namespace, version string) *args {
	config := createConfig("vm-import-config", "I am unique")
	client := createClient(config)
	reconciler := createReconciler(client, version, namespace)

	return &args{
		config:     config,
		client:     client,
		reconciler: reconciler,
	}
}

func createReconciler(client realClient.Client, version, namespace string) *ReconcileVMImportConfig {
	operatorArgs := &OperatorArgs{
		OperatorVersion:        version,
		DeployClusterResources: "true",
		ControllerImage:        "vm-import-controller",
		PullPolicy:             "Always",
		Namespace:              namespace,
	}

	r := &ReconcileVMImportConfig{
		client:       client,
		scheme:       scheme.Scheme,
		namespace:    Namespace,
		operatorArgs: operatorArgs,
	}

	mgr, err := manager.New(cfg, manager.Options{})
	Expect(err).ToNot(HaveOccurred())
	r.add(mgr)

	return r
}

func reconcileRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func doReconcile(args *args) {
	result, err := args.reconciler.Reconcile(reconcileRequest(args.config.Name))
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	args.config, err = getConfig(args.client, args.config)
	Expect(err).ToNot(HaveOccurred())
}

func getConfig(client realClient.Client, config *vmimportv1alpha1.VMImportConfig) (*vmimportv1alpha1.VMImportConfig, error) {
	result, err := getObject(client, config)
	if err != nil {
		return nil, err
	}
	return result.(*vmimportv1alpha1.VMImportConfig), nil
}

func setDeploymentsReady(args *args) bool {
	resources, err := args.reconciler.getAllResources(args.config)
	Expect(err).ToNot(HaveOccurred())
	running := false

	for _, r := range resources {
		d, ok := r.(*appsv1.Deployment)
		if !ok {
			continue
		}

		Expect(running).To(BeFalse())

		d, err := getDeployment(args.client, d)
		if err != nil {
			return running
		}
		if d.Spec.Replicas != nil {
			d.Status.Replicas = *d.Spec.Replicas
			d.Status.ReadyReplicas = d.Status.Replicas
			err = args.client.Update(context.TODO(), d)
			Expect(err).ToNot(HaveOccurred())
		}

		doReconcile(args)

		if len(args.config.Status.Conditions) == 3 &&
			conditions.IsStatusConditionTrue(args.config.Status.Conditions, conditions.ConditionAvailable) &&
			conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionProgressing) &&
			conditions.IsStatusConditionFalse(args.config.Status.Conditions, conditions.ConditionDegraded) {
			running = true
		}
	}

	return running
}

func getDeployment(client realClient.Client, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	result, err := getObject(client, deployment)
	if err != nil {
		return nil, err
	}
	return result.(*appsv1.Deployment), nil
}

func getModifiedResource(reconciler *ReconcileVMImportConfig, cr *vmimportv1alpha1.VMImportConfig, modify modifyResource, tomodify isModifySubject) (runtime.Object, runtime.Object, error) {
	resources, err := reconciler.getAllResources(cr)
	if err != nil {
		return nil, nil, err
	}

	//find the resource to modify
	var orig runtime.Object
	for _, resource := range resources {
		r, err := getObject(reconciler.client, resource)
		Expect(err).ToNot(HaveOccurred())
		if tomodify(r) {
			orig = r
			break
		}
	}
	//apply modify function on resource and return modified one
	return modify(orig)
}
