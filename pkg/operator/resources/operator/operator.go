package operator

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/blang/semver"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	vmimportmetrics "github.com/kubevirt/vm-import-operator/pkg/metrics"
	"github.com/kubevirt/vm-import-operator/pkg/utils"
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/lib/version"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	operatorName = "vm-import-operator"
	// ControllerName defines name of the controller
	ControllerName     = "vm-import-controller"
	serviceAccountName = operatorName
)

var commonLabels = map[string]string{
	"v2v.kubevirt.io": "",
}

var operatorLabels = map[string]string{
	"operator.v2v.kubevirt.io": "",
}

// ClusterServiceVersionData - Data arguments used to create vm import operator's CSV manifest
type ClusterServiceVersionData struct {
	CsvVersion         string
	ReplacesCsvVersion string
	Namespace          string
	ImagePullPolicy    string
	OperatorVersion    string
	OperatorImage      string
	ControllerImage    string
}

type csvPermissions struct {
	ServiceAccountName string              `json:"serviceAccountName"`
	Rules              []rbacv1.PolicyRule `json:"rules"`
}
type csvDeployments struct {
	Name string                `json:"name"`
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

type csvStrategySpec struct {
	ClusterPermissions []csvPermissions `json:"clusterPermissions"`
	Deployments        []csvDeployments `json:"deployments"`
}

// CreateControllerRole returns role for vm-controller-operator
func CreateControllerRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   ControllerName,
			Labels: map[string]string{},
		},
		Rules: getControllerPolicyRules(),
	}
}

// CreateControllerRoleBinding returns role binding for vm-import-operator
func CreateControllerRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ControllerName,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     ControllerName,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ControllerName,
				Namespace: namespace,
			},
		},
	}
}

func getControllerPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
				"services",
				"services/finalizers",
				"endpoints",
				"persistentvolumeclaims",
				"events",
				"configmaps",
				"secrets",
				"services",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"deployments",
				"daemonsets",
				"replicasets",
				"statefulsets",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"monitoring.coreos.com",
			},
			Resources: []string{
				"servicemonitors",
			},
			Verbs: []string{
				"get",
				"create",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			ResourceNames: []string{
				"vm-import-operator",
			},
			Resources: []string{
				"deployments/finalizers",
			},
			Verbs: []string{
				"update",
			},
		},
		{
			APIGroups: []string{
				"v2v.kubevirt.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"kubevirt.io",
			},
			Resources: []string{
				"virtualmachines",
				"virtualmachines/finalizers",
				"virtualmachineinstances",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"template.openshift.io",
			},
			Resources: []string{
				"templates",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"template.openshift.io",
			},
			Resources: []string{
				"processedtemplates",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"storage.k8s.io",
			},
			Resources: []string{
				"storageclasses",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"k8s.cni.cncf.io",
			},
			Resources: []string{
				"network-attachment-definitions",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"hco.kubevirt.io",
			},
			Resources: []string{
				"hyperconvergeds",
			},
			Verbs: []string{
				"get",
				"list",
			},
		},
	}
	return rules
}

func getOperatorPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
				"events",
				"configmaps",
				"secrets",
				"serviceaccounts",
				"services",
				"services/finalizers",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"deployments",
				"deployments/finalizers",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"v2v.kubevirt.io",
			},
			Resources: []string{
				"vmimportconfigs",
				"vmimportconfigs/finalizers",
				"vmimportconfigs/status",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"apiextensions.k8s.io",
			},
			Resources: []string{
				"customresourcedefinitions",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"rbac.authorization.k8s.io",
			},
			Resources: []string{
				"clusterrolebindings",
				"clusterroles",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"monitoring.coreos.com",
			},
			Resources: []string{
				"servicemonitors",
			},
			Verbs: []string{
				"*",
			},
		},
	}
	return rules
}

// CreateControllerDeployment returns vmimport controller deployment
func CreateControllerDeployment(name, namespace, image, pullPolicy string, numReplicas int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *createControllerDeploymentSpec(image, pullPolicy, "v2v.kubevirt.io", ControllerName, numReplicas),
	}
	return deployment
}

func createControllerDeploymentSpec(image string, pullPolicy string, matchKey string, matchValue string, numReplicas int32) *appsv1.DeploymentSpec {
	matchMap := map[string]string{matchKey: matchValue}
	return &appsv1.DeploymentSpec{
		Replicas: &numReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: utils.WithLabels(matchMap, operatorLabels),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.WithLabels(matchMap, operatorLabels),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: ControllerName,
				Containers:         createControllerContainers(image, pullPolicy),
			},
		},
	}
}

func createControllerContainers(image string, pullPolicy string) []v1.Container {
	return []corev1.Container{
		{
			Name:  ControllerName,
			Image: image,
			Command: []string{
				ControllerName,
			},
			ImagePullPolicy: corev1.PullPolicy(pullPolicy),
			Env:             createControllerEnv(pullPolicy),
		},
	}
}

func createControllerEnv(pullPolicy string) []v1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "WATCH_NAMESPACE",
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "OPERATOR_NAME",
			Value: "vm-import-operator",
		},
		{
			Name:  "IMPORT_POD_RESTART_TOLERANCE",
			Value: "3",
		},
		{
			Name:  "PULL_POLICY",
			Value: pullPolicy,
		},
	}
}

// CreateVMImportConfig creates the VMImportConfig CRD
func CreateVMImportConfig() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "vmimportconfigs.v2v.kubevirt.io",
			Labels: map[string]string{
				"operator.v2v.kubevirt.io": "",
			},
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "v2v.kubevirt.io",
			Scope: "Cluster",
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "VMImportConfig is the Schema for the vmimportconfigs API",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Description: "APIVersion defines the versioned schema of this representation of an object",
									Type:        "string",
								},
								"kind": {
									Description: "Kind is a string value representing the REST resource this object represents",
									Type:        "string",
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Description: "VMImportConfigSpec defines the desired state of VMImportConfig",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"imagePullPolicy": {
											Type: "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"Always"`),
												},
												{
													Raw: []byte(`"IfNotPresent"`),
												},
												{
													Raw: []byte(`"Never"`),
												},
											},
										},
									},
								},
								"status": {
									Description: "VMImportConfigStatus defines the observed state of VMImportConfig",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Description: "A list of current conditions of the VMImportConfig resource",
											Type:        "array",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Description: "Last time the state of the condition was checked",
															Type:        "string",
															Format:      "date-time",
														},
														"lastTransitionTime": {
															Description: "Last time the state of the condition changed",
															Type:        "string",
															Format:      "date-time",
														},
														"message": {
															Description: "Message related to the last condition change",
															Type:        "string",
														},
														"reason": {
															Description: "Reason the last condition changed",
															Type:        "string",
														},
														"status": {
															Description: "Current status of the condition, True, False, Unknown",
															Type:        "string",
														},
													},
												},
											},
										},
										"targetVersion": {
											Description: "The desired version of the VMImportConfig resource",
											Type:        "string",
										},
										"observedVersion": {
											Description: "The observed version of the VMImportConfig resource",
											Type:        "string",
										},
										"operatorVersion": {
											Description: "The version of the VMImportConfig resource as defined by the operator",
											Type:        "string",
										},
									},
								},
							},
						},
					},
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "VMImportConfig is the Schema for the vmimportconfigs API",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Description: "APIVersion defines the versioned schema of this representation of an object",
									Type:        "string",
								},
								"kind": {
									Description: "Kind is a string value representing the REST resource this object represents",
									Type:        "string",
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Description: "VMImportConfigSpec defines the desired state of VMImportConfig",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"imagePullPolicy": {
											Type: "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"Always"`),
												},
												{
													Raw: []byte(`"IfNotPresent"`),
												},
												{
													Raw: []byte(`"Never"`),
												},
											},
										},
									},
								},
								"status": {
									Description: "VMImportConfigStatus defines the observed state of VMImportConfig",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Description: "A list of current conditions of the VMImportConfig resource",
											Type:        "array",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Description: "Last time the state of the condition was checked",
															Type:        "string",
															Format:      "date-time",
														},
														"lastTransitionTime": {
															Description: "Last time the state of the condition changed",
															Type:        "string",
															Format:      "date-time",
														},
														"message": {
															Description: "Message related to the last condition change",
															Type:        "string",
														},
														"reason": {
															Description: "Reason the last condition changed",
															Type:        "string",
														},
														"status": {
															Description: "Current status of the condition, True, False, Unknown",
															Type:        "string",
														},
														"type": {
															Description: "ConditionType is the state of the operator's reconciliation functionality.",
															Type:        "string",
														},
													},
												},
											},
										},
										"targetVersion": {
											Description: "The desired version of the VMImportConfig resource",
											Type:        "string",
										},
										"observedVersion": {
											Description: "The observed version of the VMImportConfig resource",
											Type:        "string",
										},
										"operatorVersion": {
											Description: "The version of the VMImportConfig resource as defined by the operator",
											Type:        "string",
										},
										"phase": {
											Description: "VMImportPhase is the current phase of the VMImport deployment",
											Type:        "string",
										},
									},
								},
							},
						},
					},
				},
			},
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "VMImportConfig",
				ListKind: "VMImportConfigList",
				Plural:   "vmimportconfigs",
				Singular: "vmimportconfig",
				Categories: []string{
					"all",
				},
			},
		},
	}
}

// CreateVMImport creates the VM Import CRD
func CreateVMImport() *extv1.CustomResourceDefinition {
	maxTargetVMName := int64(validation.LabelValueMaxLength)
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "virtualmachineimports.v2v.kubevirt.io",
			Labels: map[string]string{
				"operator.v2v.kubevirt.io": "",
			},
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "v2v.kubevirt.io",
			Scope: "Namespaced",
			Conversion: &extv1.CustomResourceConversion{
				Strategy: extv1.NoneConverter,
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Type: "string",
									Description: `APIVersion defines the versioned schema of this representation
					of an object. Servers should convert recognized schemas to the latest
					internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources`,
								},
								"kind": {
									Type: "string",
									Description: `Kind is a string value representing the REST resource this
					object represents. Servers may infer this from the endpoint the client
					submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds`,
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Type:        "object",
									Description: "VirtualMachineImportSpec defines the desired state of VirtualMachineImport",
									Properties: map[string]extv1.JSONSchemaProps{
										"providerCredentialsSecret": {
											Type:        "object",
											Description: "ProviderCredentialsSecret defines how a secret resource should be identified on kubevirt",
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type: "string",
												},
												"namespace": {
													Type: "string",
												},
											},
											Required: []string{"name"},
										},
										"resourceMapping": {
											Type:        "object",
											Description: "ObjectIdentifier defines how a resource should be identified",
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type: "string",
												},
												"namespace": {
													Type: "string",
												},
											},
											Required: []string{"name"},
										},
										"source": {
											Type:        "object",
											Description: "VirtualMachineImportSourceSpec defines the source provider and the internal mapping resources",
											Properties: map[string]extv1.JSONSchemaProps{
												"ovirt": {
													Type:        "object",
													Description: `VirtualMachineImportOvirtSourceSpec defines the mapping resources and the VM identity for oVirt source provider`,
													Properties: map[string]extv1.JSONSchemaProps{
														"mappings": {
															Type:        "object",
															Description: "OvirtMappings defines the mappings of ovirt resources to kubevirt",
															Properties: map[string]extv1.JSONSchemaProps{
																"networkMappings": {
																	Type: "array",
																	Description: `NetworkMappings defines the mapping of vnic profile
					to network attachment definition When providing source network by name, the format is 'network name/vnic profile name'. When
					providing source network by ID, the ID represents the vnic profile ID. A logical network from ovirt can be mapped to multiple network
					attachment definitions on kubevirt by using vnic profile to network attachment definition mapping.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
																"storageMappings": {
																	Type:        "array",
																	Description: `StorageMappings defines the mapping of storage domains to storage classes.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
																"diskMappings": {
																	Type: "array",
																	Description: `DiskMappings defines the mapping of disks to storage
					classes DiskMappings.Source.ID represents the disk ID on ovirt (as opposed to disk-attachment ID) DiskMappings.Source.Name represents
					the disk alias on ovirt DiskMappings is respected only when provided in context of a single VM import within VirtualMachineImport.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
															},
														},
														"vm": {
															Type:        "object",
															Description: `VirtualMachineImportOvirtSourceVMSpec defines the definition of the VM info in oVirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"cluster": {
																	Description: `VirtualMachineImportOvirtSourceVMClusterSpec defines the source cluster's identity of the VM in oVirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"id": {
																	Type: "string",
																},
																"name": {
																	Type: "string",
																},
															},
														},
													},
													Required: []string{"vm"},
												},
											},
										},
										"startVm": {
											Type: "boolean",
										},
										"targetVmName": {
											Type:      "string",
											MaxLength: &maxTargetVMName,
										},
									},
									Required: []string{"providerCredentialsSecret", "source"},
								},
								"status": {
									Type:        "object",
									Description: `VirtualMachineImportStatus defines the observed state of VirtualMachineImport`,
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Description: "A list of current conditions of the VirtualMachineImport resource",
											Type:        "array",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Description: "The last time we got an update on a given condition",
															Type:        "string",
															Format:      "date-time",
														},
														"lastTransitionTime": {
															Description: `The last time the condition transit from one status to another`,
															Type:        "string",
															Format:      "date-time",
														},
														"message": {
															Description: `A human-readable message indicating details about last transition`,
															Type:        "string",
														},
														"reason": {
															Description: `A brief CamelCase string that describes why the VM import process is in current condition status`,
															Type:        "string",
														},
														"status": {
															Description: "Status of the condition, one of True, False, Unknown",
															Type:        "string",
														},
														"type": {
															Description: "Type of virtual machine import condition",
															Type:        "string",
														},
													},
													Required: []string{"status", "type"},
												},
											},
										},
										"dataVolumes": {
											Description: `DataVolumeItem defines the details of a data volume created by the VM import process`,
											Type:        "array",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Type: "string",
														},
													},
													Required: []string{"name"},
												},
											},
										},
										"targetVmName": {
											Description: "The name of the virtual machine created by the import process",
											Type:        "string",
										},
									},
								},
							},
						},
					},
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Type: "string",
									Description: `APIVersion defines the versioned schema of this representation
		of an object. Servers should convert recognized schemas to the latest
		internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources`,
								},
								"kind": {
									Type: "string",
									Description: `Kind is a string value representing the REST resource this
		object represents. Servers may infer this from the endpoint the client
		submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds`,
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Type:        "object",
									Description: "VirtualMachineImportSpec defines the desired state of VirtualMachineImport",
									Properties: map[string]extv1.JSONSchemaProps{
										"providerCredentialsSecret": {
											Type:        "object",
											Description: "ProviderCredentialsSecret defines how a secret resource should be identified on kubevirt",
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Description: "Name of the secret to be used for the virtual machine import",
													Type:        "string",
												},
												"namespace": {
													Description: "Namespace of the secret to be used for the virtual machine import",
													Type:        "string",
												},
											},
											Required: []string{"name"},
										},
										"resourceMapping": {
											Type:        "object",
											Description: "ObjectIdentifier defines how a resource should be identified",
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Description: "Name of the ResourceMapping to be used for the virtual machine import",
													Type:        "string",
												},
												"namespace": {
													Description: "Namespace of the ResourceMapping to be used for the virtual machine import",
													Type:        "string",
												},
											},
											Required: []string{"name"},
										},
										"source": {
											Type:        "object",
											Description: "VirtualMachineImportSourceSpec defines the source provider and the internal mapping resources",
											Properties: map[string]extv1.JSONSchemaProps{
												"ovirt": {
													Type:        "object",
													Description: `VirtualMachineImportOvirtSourceSpec defines the mapping resources and the VM identity for oVirt source provider`,
													Properties: map[string]extv1.JSONSchemaProps{
														"mappings": {
															Type:        "object",
															Description: "OvirtMappings defines the mappings of ovirt resources to kubevirt",
															Properties: map[string]extv1.JSONSchemaProps{
																"networkMappings": {
																	Type: "array",
																	Description: `NetworkMappings defines the mapping of vnic profile
		to network attachment definition When providing source network by name, the format is 'network name/vnic profile name'. When
		providing source network by ID, the ID represents the vnic profile ID. A logical network from ovirt can be mapped to multiple network
		attachment definitions on kubevirt by using vnic profile to network attachment definition mapping.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
																"storageMappings": {
																	Type:        "array",
																	Description: `StorageMappings defines the mapping of storage domains to storage classes.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `StorageResourceMappingItem defines the mapping of a single storage resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																				"volumeMode": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
																"diskMappings": {
																	Type: "array",
																	Description: `DiskMappings defines the mapping of disks to storage
		classes DiskMappings.Source.ID represents the disk ID on ovirt (as opposed to disk-attachment ID) DiskMappings.Source.Name represents
		the disk alias on ovirt DiskMappings is respected only when provided in context of a single VM import within VirtualMachineImport.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `NetworkResourceMappingItem defines the mapping of a single disk resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																				"volumeMode": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
															},
														},
														"vm": {
															Type:        "object",
															Description: `VirtualMachineImportOvirtSourceVMSpec defines the definition of the VM info in oVirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"cluster": {
																	Description: `VirtualMachineImportOvirtSourceVMClusterSpec defines the source cluster's identity of the VM in oVirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Description: `id defines the id of the source cluster where VM resides`,
																			Type:        "string",
																		},
																		"name": {
																			Description: `name defines the name of the source cluster where VM resides`,
																			Type:        "string",
																		},
																	},
																},
																"id": {
																	Description: `id defines the id of the source VM`,
																	Type:        "string",
																},
																"name": {
																	Description: `name defines the name of the source VM`,
																	Type:        "string",
																},
															},
														},
													},
													Required: []string{"vm"},
												},
												"vmware": {
													Type:        "object",
													Description: `VirtualMachineImportVmwareSourceSpec defines the mapping resources and the VM identity for vmware source provider`,
													Properties: map[string]extv1.JSONSchemaProps{
														"mappings": {
															Type:        "object",
															Description: "VmwareMappings defines the mappings of vmware resources to kubevirt",
															Properties: map[string]extv1.JSONSchemaProps{
																"networkMappings": {
																	Type: "array",
																	Description: `NetworkMappings defines the mapping of guest network interfaces to kubevirt networks
NetworkMappings.Source.Name represents the network field of the GuestNicInfo in vCenter
NetworkMappings.Source.ID represents the macAddress field of the network adapter`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `NetworkResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
																"storageMappings": {
																	Type:        "array",
																	Description: `StorageMappings defines the mapping of storage domains to storage classes.`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `StorageResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																				"volumeMode": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
																"diskMappings": {
																	Type: "array",
																	Description: `DiskMappings defines the mapping of VirtualDisks to storage classes.
DiskMappings.Source.Name represents the disk name in vCenter
DiskMappings.Source.ID represents the DiskObjectId or vDiskID of the VirtualDisk in vCenter`,
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Type:        "object",
																			Description: `StorageResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																			Properties: map[string]extv1.JSONSchemaProps{
																				"source": {
																					Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"id": {
																							Type: "string",
																						},
																						"name": {
																							Type: "string",
																						},
																					},
																				},
																				"target": {
																					Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"name": {
																							Type: "string",
																						},
																						"namespace": {
																							Type: "string",
																						},
																					},
																					Required: []string{"name"},
																				},
																				"type": {
																					Type: "string",
																				},
																				"volumeMode": {
																					Type: "string",
																				},
																			},
																			Required: []string{"source"},
																		},
																	},
																},
															},
														},
														"vm": {
															Type:        "object",
															Description: `VirtualMachineImportVmwareSourceVMSpec defines how to identify the VM in vCenter`,
															Properties: map[string]extv1.JSONSchemaProps{
																"id": {
																	Type: "string",
																},
																"name": {
																	Type: "string",
																},
															},
														},
													},
													Required: []string{"vm"},
												},
											},
										},
										"startVm": {
											Type:        "boolean",
											Description: `If true imported virtual machine will be started`,
										},
										"targetVmName": {
											Description: `Specifies the name of the imported virtual machine`,
											Type:        "string",
											MaxLength:   &maxTargetVMName,
										},
									},
									Required: []string{"providerCredentialsSecret", "source"},
								},
								"status": {
									Type:        "object",
									Description: `VirtualMachineImportStatus defines the observed state of VirtualMachineImport`,
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Description: "A list of current conditions of the VirtualMachineImport resource",
											Type:        "array",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Description: "The last time we got an update on a given condition",
															Type:        "string",
															Format:      "date-time",
														},
														"lastTransitionTime": {
															Description: `The last time the condition transit from one status to another`,
															Type:        "string",
															Format:      "date-time",
														},
														"message": {
															Description: `A human-readable message indicating details about last transition`,
															Type:        "string",
														},
														"reason": {
															Description: `A brief CamelCase string that describes why the VM import process is in current condition status`,
															Type:        "string",
														},
														"status": {
															Description: "Status of the condition, one of True, False, Unknown",
															Type:        "string",
														},
														"type": {
															Description: "Type of virtual machine import condition",
															Type:        "string",
														},
													},
													Required: []string{"status", "type"},
												},
											},
										},
										"dataVolumes": {
											Description: `DataVolumeItem defines the details of a data volume created by the VM import process`,
											Type:        "array",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Type: "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"name": {
															Description: `Name of the DataVolume that was created by virtual machine import`,
															Type:        "string",
														},
													},
													Required: []string{"name"},
												},
											},
										},
										"targetVmName": {
											Description: "The name of the virtual machine created by the import process",
											Type:        "string",
										},
									},
								},
							},
						},
					},
				},
			},
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "VirtualMachineImport",
				ListKind: "VirtualMachineImportList",
				Plural:   "virtualmachineimports",
				Singular: "virtualmachineimport",
				Categories: []string{
					"all",
				},
				ShortNames: []string{"vmimports"},
			},
		},
	}
}

// CreateResourceMapping creates the ResourceMapping CRD
func CreateResourceMapping() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "resourcemappings.v2v.kubevirt.io",
			Labels: map[string]string{
				"operator.v2v.kubevirt.io": "",
			},
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "v2v.kubevirt.io",
			Scope: "Namespaced",
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Type: "string",
									Description: `APIVersion defines the versioned schema of this representation
		of an object. Servers should convert recognized schemas to the latest
		internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources`,
								},
								"kind": {
									Type: "string",
									Description: `Kind is a string value representing the REST resource this
		object represents. Servers may infer this from the endpoint the client
		submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds`,
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Type:        "object",
									Description: "ResourceMappingSpec defines the desired state of ResourceMapping",
									Properties: map[string]extv1.JSONSchemaProps{
										"ovirt": {
											Type:        "object",
											Description: "OvirtMappings defines the mappings of ovirt resources to kubevirt",
											Properties: map[string]extv1.JSONSchemaProps{
												"networkMappings": {
													Type: "array",
													Description: `NetworkMappings defines the mapping of vnic profile
		to network attachment definition When providing source network by name, the format is 'network name/vnic profile name'.
		When providing source network by ID, the ID represents the vnic profile ID. A logical network from ovirt can be mapped
		to multiple network attachment definitions on kubevirt by using vnic profile to network attachment definition mapping.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
												"storageMappings": {
													Type:        "array",
													Description: `StorageMappings defines the mapping of storage domains to storage classes.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `StorageResourceMappingItem defines the mapping of a single storage resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
												"diskMappings": {
													Type: "array",
													Description: `DiskMappings defines the mapping of disks to storage
		classes DiskMappings.Source.ID represents the disk ID on ovirt (as opposed to disk-attachment ID) DiskMappings.Source.Name represents
		the disk alias on ovirt DiskMappings is respected only when provided in context of a single VM import within VirtualMachineImport.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `NetworkResourceMappingItem defines the mapping of a single disk resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
											},
										},
									},
								},
								"status": {
									Description: "ResourceMappingStatus defines the observed state of ResourceMapping",
									Type:        "object",
								},
							},
						},
					},
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Type: "string",
									Description: `APIVersion defines the versioned schema of this representation
		of an object. Servers should convert recognized schemas to the latest
		internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources`,
								},
								"kind": {
									Type: "string",
									Description: `Kind is a string value representing the REST resource this
		object represents. Servers may infer this from the endpoint the client
		submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds`,
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Type:        "object",
									Description: "ResourceMappingSpec defines the desired state of ResourceMapping",
									Properties: map[string]extv1.JSONSchemaProps{
										"ovirt": {
											Type:        "object",
											Description: "OvirtMappings defines the mappings of ovirt resources to kubevirt",
											Properties: map[string]extv1.JSONSchemaProps{
												"networkMappings": {
													Type: "array",
													Description: `NetworkMappings defines the mapping of vnic profile
		to network attachment definition When providing source network by name, the format is 'network name/vnic profile name'.
		When providing source network by ID, the ID represents the vnic profile ID. A logical network from ovirt can be mapped
		to multiple network attachment definitions on kubevirt by using vnic profile to network attachment definition mapping.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
												"storageMappings": {
													Type:        "array",
													Description: `StorageMappings defines the mapping of storage domains to storage classes.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `StorageResourceMappingItem defines the mapping of a single storage resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
																"volumeMode": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
												"diskMappings": {
													Type: "array",
													Description: `DiskMappings defines the mapping of disks to storage
		classes DiskMappings.Source.ID represents the disk ID on ovirt (as opposed to disk-attachment ID) DiskMappings.Source.Name represents
		the disk alias on ovirt DiskMappings is respected only when provided in context of a single VM import within VirtualMachineImport.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `NetworkResourceMappingItem defines the mapping of a single disk resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
																"volumeMode": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
											},
										},
										"vmware": {
											Type:        "object",
											Description: "VmwareMappings defines the mappings of vmware resources to kubevirt",
											Properties: map[string]extv1.JSONSchemaProps{
												"networkMappings": {
													Type: "array",
													Description: `NetworkMappings defines the mapping of guest network interfaces to kubevirt networks
NetworkMappings.Source.Name represents the network field of the GuestNicInfo in vCenter
NetworkMappings.Source.ID represents the macAddress field of the network adapter`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `NetworkResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
												"storageMappings": {
													Type:        "array",
													Description: `StorageMappings defines the mapping of storage domains to storage classes.`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `StorageResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
																"volumeMode": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
												"diskMappings": {
													Type: "array",
													Description: `DiskMappings defines the mapping of VirtualDisks to storage classes.
DiskMappings.Source.Name represents the disk name in vCenter
DiskMappings.Source.ID represents the DiskObjectId or vDiskID of the VirtualDisk in vCenter`,
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type:        "object",
															Description: `StorageResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
															Properties: map[string]extv1.JSONSchemaProps{
																"source": {
																	Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"id": {
																			Type: "string",
																		},
																		"name": {
																			Type: "string",
																		},
																	},
																},
																"target": {
																	Description: `ObjectIdentifier defines how a resource should be identified on kubevirt`,
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"name": {
																			Type: "string",
																		},
																		"namespace": {
																			Type: "string",
																		},
																	},
																	Required: []string{"name"},
																},
																"type": {
																	Type: "string",
																},
																"volumeMode": {
																	Type: "string",
																},
															},
															Required: []string{"source", "target"},
														},
													},
												},
											},
										},
									},
								},
								"status": {
									Description: "ResourceMappingStatus defines the observed state of ResourceMapping",
									Type:        "object",
								},
							},
						},
					},
				},
			},
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "ResourceMapping",
				ListKind: "ResourceMappingList",
				Plural:   "resourcemappings",
				Singular: "resourcemapping",
				Categories: []string{
					"all",
				},
			},
		},
	}
}

func createOperatorDeployment(operatorVersion, namespace, deployClusterResources, operatorImage, controllerImage, pullPolicy string) *appsv1.Deployment {
	deployment := CreateOperatorDeployment(operatorName, namespace, "name", operatorName, serviceAccountName, int32(1))
	container := CreateContainer(operatorName, operatorImage, corev1.PullPolicy(pullPolicy))
	container.Env = createOperatorEnvVar(operatorVersion, deployClusterResources, controllerImage, pullPolicy)
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	return deployment
}

// CreateOperatorDeployment creates deployment
func CreateOperatorDeployment(name, namespace, matchKey, matchValue, serviceAccount string, numReplicas int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *CreateOperatorDeploymentSpec(matchKey, matchValue, serviceAccount, numReplicas),
	}
	return deployment
}

// CreateContainer creates container
func CreateContainer(name, image string, pullPolicy corev1.PullPolicy) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: pullPolicy,
	}
}

func createOperatorEnvVar(operatorVersion, deployClusterResources, controllerImage, pullPolicy string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "DEPLOY_CLUSTER_RESOURCES",
			Value: deployClusterResources,
		},
		{
			Name:  "OPERATOR_VERSION",
			Value: operatorVersion,
		},
		{
			Name:  "CONTROLLER_IMAGE",
			Value: controllerImage,
		},
		{
			Name:  "PULL_POLICY",
			Value: pullPolicy,
		},
		{
			Name: "WATCH_NAMESPACE",
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}
}

// CreateOperatorDeploymentSpec creates deployment
func CreateOperatorDeploymentSpec(matchKey, matchValue, serviceAccount string, numReplicas int32) *appsv1.DeploymentSpec {
	matchMap := map[string]string{matchKey: matchValue}
	spec := &appsv1.DeploymentSpec{
		Replicas: &numReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: utils.WithLabels(matchMap, operatorLabels),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.WithLabels(matchMap, operatorLabels),
			},
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: &[]bool{true}[0],
				},
				ServiceAccountName: serviceAccount,
			},
		},
	}

	return spec
}

// CreateServiceAccount creates service account
func CreateServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ControllerName,
			Labels:    utils.WithLabels(nil, commonLabels),
			Namespace: namespace,
		},
	}
}

// CreateMetricsService create a Service resource for metrics
func CreateMetricsService(namespace string) *v1.Service {
	servicePorts := []v1.ServicePort{
		{Port: vmimportmetrics.MetricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: vmimportmetrics.MetricsPort}},
		{Port: vmimportmetrics.OperatorMetricsPort, Name: metrics.CRPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: vmimportmetrics.OperatorMetricsPort}},
	}
	labels := map[string]string{"name": operatorName}

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", operatorName),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1.ServiceSpec{
			Ports:    servicePorts,
			Selector: map[string]string{"v2v.kubevirt.io": "vm-import-controller"},
		},
	}
}

// CreateServiceMonitor create a service monitor for vm-operator metrics
func CreateServiceMonitor(monitoringNamespace string, svcNamespace string) *monitoringv1.ServiceMonitor {
	labels := map[string]string{"name": operatorName}
	var endpoints []monitoringv1.Endpoint
	for _, name := range []string{metrics.OperatorPortName, metrics.CRPortName} {
		endpoints = append(endpoints, monitoringv1.Endpoint{Port: name})
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-monitor", operatorName),
			Namespace: monitoringNamespace,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{svcNamespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: endpoints,
		},
	}
}

// NewClusterServiceVersion creates all cluster resources fr a specific group/component
func NewClusterServiceVersion(data *ClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {
	deployment := createOperatorDeployment(
		data.OperatorVersion,
		data.Namespace,
		"true",
		data.OperatorImage,
		data.ControllerImage,
		data.ImagePullPolicy,
	)

	strategySpec := csvStrategySpec{
		ClusterPermissions: []csvPermissions{
			{
				ServiceAccountName: serviceAccountName,
				Rules:              getOperatorPolicyRules(),
			},
		},
		Deployments: []csvDeployments{
			{
				Name: operatorName,
				Spec: deployment.Spec,
			},
		},
	}

	strategySpecJSONBytes, err := json.Marshal(strategySpec)
	if err != nil {
		return nil, err
	}

	csvVersion, err := semver.New(data.CsvVersion)
	if err != nil {
		return nil, err
	}

	return &csvv1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterServiceVersion",
			APIVersion: "operators.coreos.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName + "." + data.CsvVersion,
			Namespace: data.Namespace,
			Annotations: map[string]string{
				"capabilities": "Virtual Machine Import",
				"categories":   "Import,Virtualization, RHV",
				"alm-examples": `
      [
        {
          "apiVersion":"v2v.kubevirt.io/v1beta1",
          "kind":"VMImportConfig",
          "metadata": {
            "name":"vm-import-operator-config"
          },
          "spec": {
            "imagePullPolicy":"IfNotPresent"
          }
        }
      ]`,
			},
		},
		Spec: csvv1.ClusterServiceVersionSpec{
			DisplayName: "VM import operator",
			Description: "VM import operator provides ability to import virtual machines from other infrastructure like oVirt/RHV",
			Keywords:    []string{"Import", "Virtualization", "oVirt", "RHV"},
			Version:     version.OperatorVersion{Version: *csvVersion},
			Maturity:    "alpha",
			Replaces:    data.ReplacesCsvVersion,
			Maintainers: []csvv1.Maintainer{{
				Name:  "KubeVirt project",
				Email: "kubevirt-dev@googlegroups.com",
			}},
			Provider: csvv1.AppLink{
				Name: "KubeVirt project",
			},
			Links: []csvv1.AppLink{
				{
					Name: "VM import operator",
					URL:  "https://github.com/kubevirt/vm-import-operator/blob/master/README.md",
				},
				{
					Name: "Source Code",
					URL:  "https://github.com/kubevirt/vm-import-operator",
				},
			},
			Labels: map[string]string{
				"operated-by": operatorName,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operated-by": operatorName,
				},
			},
			InstallModes: []csvv1.InstallMode{
				{
					Type:      csvv1.InstallModeTypeOwnNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeSingleNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeAllNamespaces,
					Supported: true,
				},
			},
			InstallStrategy: csvv1.NamedInstallStrategy{
				StrategyName:    "deployment",
				StrategySpecRaw: json.RawMessage(strategySpecJSONBytes),
			},
			CustomResourceDefinitions: csvv1.CustomResourceDefinitions{
				Owned: []csvv1.CRDDescription{
					{
						Name:        "vmimportconfigs.v2v.kubevirt.io",
						Version:     "v1beta1",
						Kind:        "VMImportConfig",
						DisplayName: "Virtual Machine import config",
						Description: "Represents a virtual machine import config",
						SpecDescriptors: []csvv1.SpecDescriptor{
							{
								Description:  "The ImageRegistry to use for vm import.",
								DisplayName:  "ImageRegistry",
								Path:         "imageRegistry",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The ImageTag to use for vm import.",
								DisplayName:  "ImageTag",
								Path:         "imageTag",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The ImagePullPolicy to use for vm import.",
								DisplayName:  "ImagePullPolicy",
								Path:         "imagePullPolicy",
								XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:imagePullPolicy"},
							},
						},
						StatusDescriptors: []csvv1.StatusDescriptor{
							{
								Description:  "The deployment phase.",
								DisplayName:  "Phase",
								Path:         "phase",
								XDescriptors: []string{"urn:alm:descriptor:io.kubernetes.phase"},
							},
							{
								Description:  "Explanation for the current status of the vm import deployment.",
								DisplayName:  "Conditions",
								Path:         "conditions",
								XDescriptors: []string{"urn:alm:descriptor:io.kubernetes.conditions"},
							},
							{
								Description:  "The observed version of the vm import deployment.",
								DisplayName:  "Observed vm import Version",
								Path:         "observedVersion",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The targeted version of the vm import deployment.",
								DisplayName:  "Target vm import Version",
								Path:         "targetVersion",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The version of the vm import Operator",
								DisplayName:  "Vm import Operator Version",
								Path:         "operatorVersion",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
						},
					},
				},
			},
		},
	}, nil
}
