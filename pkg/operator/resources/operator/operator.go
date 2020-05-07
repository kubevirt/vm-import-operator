package operator

import (
	"encoding/json"

	"github.com/blang/semver"
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/lib/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	operatorName       = "vm-import-operator"
	serviceAccountName = operatorName
	roleName           = operatorName
)

var operatorLabels = map[string]string{
	operatorName: "",
}

// ClusterServiceVersionData - Data arguments used to create vm import operator's CSV manifest
type ClusterServiceVersionData struct {
	CsvVersion         string
	ReplacesCsvVersion string
	Namespace          string
	ImagePullPolicy    string
	OperatorVersion    string
	OperatorImage      string
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

func createRole(name string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
	}
}

func getClusterPolicyRules() []rbacv1.PolicyRule {
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
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"replicasets",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"v2v.kubevirt.io",
			},
			Resources: []string{
				"*",
				"resourcemappings",
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
	}
	return rules
}

func createOperatorDeployment(name string, namespace string, image string, pullPolicy string, matchKey string, matchValue string, numReplicas int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *createOperatorDeploymentSpec(name, image, pullPolicy, matchKey, matchValue, numReplicas),
	}
	return deployment
}

func createOperatorDeploymentSpec(name string, image string, pullPolicy string, matchKey string, matchValue string, numReplicas int32) *appsv1.DeploymentSpec {
	matchMap := map[string]string{matchKey: matchValue}
	return &appsv1.DeploymentSpec{
		Replicas: &numReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: withOperatorLabels(matchMap),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: withOperatorLabels(matchMap),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers:         createContainers(name, image, corev1.PullPolicy(pullPolicy)),
			},
		},
	}
}

func createContainers(name string, image string, pullPolicy corev1.PullPolicy) []corev1.Container {
	return []corev1.Container{
		corev1.Container{
			Name:  name,
			Image: image,
			Command: []string{
				operatorName,
			},
			ImagePullPolicy: pullPolicy,
			Env:             createEnv(name),
		},
	}
}

func createEnv(name string) []corev1.EnvVar {
	return []corev1.EnvVar{
		corev1.EnvVar{
			Name: "WATCH_NAMESPACE",
		},
		corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		corev1.EnvVar{
			Name:  "OPERATOR_NAME",
			Value: name,
		},
	}
}

// NewCrds creates crds
func NewCrds() []*extv1beta1.CustomResourceDefinition {
	return []*extv1beta1.CustomResourceDefinition{
		CreateResourceMapping(),
		CreateVMImport(),
		CreateVMImportConfig(),
	}
}

// CreateVMImportConfig creates the VMImportConfig CRD
func CreateVMImportConfig() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "vmimportconfigs.v2v.kubevirt.io",
			Labels: map[string]string{
				"operator.v2v.kubevirt.io": "",
			},
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group:   "v2v.kubevirt.io",
			Version: "v1alpha1",
			Scope:   "Namespaced",
			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
				},
			},
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:     "VMImportConfig",
				ListKind: "VMImportConfigList",
				Plural:   "vmimportconfigs",
				Singular: "vmimportconfig",
				Categories: []string{
					"all",
				},
			},
			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Description: "VMImportConfig is the Schema for the vmimportconfigs API",
					Type:        "object",
					Properties: map[string]extv1beta1.JSONSchemaProps{
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
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"imagePullPolicy": {
									Type: "string",
									Enum: []extv1beta1.JSON{
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
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"conditions": {
									Description: "A list of current conditions of the VMImportConfig resource",
									Type:        "array",
									Items: &extv1beta1.JSONSchemaPropsOrArray{
										Schema: &extv1beta1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
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
	}
}

// CreateVMImport creates the VM Import CRD
func CreateVMImport() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "virtualmachineimports.v2v.kubevirt.io",
			Labels: map[string]string{
				"operator.v2v.kubevirt.io": "",
			},
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group:   "v2v.kubevirt.io",
			Version: "v1alpha1",
			Scope:   "Namespaced",
			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
				},
			},
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:     "VirtualMachineImport",
				ListKind: "VirtualMachineImportList",
				Plural:   "virtualmachineimports",
				Singular: "virtualmachineimport",
				Categories: []string{
					"all",
				},
				ShortNames: []string{"vmimports"},
			},
			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]extv1beta1.JSONSchemaProps{
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
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"providerCredentialsSecret": {
									Type:        "object",
									Description: "ProviderCredentialsSecret defines how a secret resource should be identified on kubevirt",
									Properties: map[string]extv1beta1.JSONSchemaProps{
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
									Properties: map[string]extv1beta1.JSONSchemaProps{
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
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"ovirt": {
											Type:        "object",
											Description: `VirtualMachineImportOvirtSourceSpec defines the mapping resources and the VM identity for oVirt source provider`,
											Properties: map[string]extv1beta1.JSONSchemaProps{
												"mappings": {
													Type:        "object",
													Description: "OvirtMappings defines the mappings of ovirt resources to kubevirt",
													Properties: map[string]extv1beta1.JSONSchemaProps{
														"networkMappings": {
															Type: "array",
															Description: `NetworkMappings defines the mapping of vnic profile
to network attachment definition When providing source network by name, the format is 'network name/vnic profile name'. When
providing source network by ID, the ID represents the vnic profile ID. A logical network from ovirt can be mapped to multiple network
attachment definitions on kubevirt by using vnic profile to network attachment definition mapping.`,
															Items: &extv1beta1.JSONSchemaPropsOrArray{
																Schema: &extv1beta1.JSONSchemaProps{
																	Type:        "object",
																	Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																	Properties: map[string]extv1beta1.JSONSchemaProps{
																		"source": {
																			Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																			Type:        "object",
																			Properties: map[string]extv1beta1.JSONSchemaProps{
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
																			Properties: map[string]extv1beta1.JSONSchemaProps{
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
															Items: &extv1beta1.JSONSchemaPropsOrArray{
																Schema: &extv1beta1.JSONSchemaProps{
																	Type:        "object",
																	Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																	Properties: map[string]extv1beta1.JSONSchemaProps{
																		"source": {
																			Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																			Type:        "object",
																			Properties: map[string]extv1beta1.JSONSchemaProps{
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
																			Properties: map[string]extv1beta1.JSONSchemaProps{
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
															Items: &extv1beta1.JSONSchemaPropsOrArray{
																Schema: &extv1beta1.JSONSchemaProps{
																	Type:        "object",
																	Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
																	Properties: map[string]extv1beta1.JSONSchemaProps{
																		"source": {
																			Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
																			Type:        "object",
																			Properties: map[string]extv1beta1.JSONSchemaProps{
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
																			Properties: map[string]extv1beta1.JSONSchemaProps{
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
												"vm": {
													Type:        "object",
													Description: `VirtualMachineImportOvirtSourceVMSpec defines the definition of the VM info in oVirt`,
													Properties: map[string]extv1beta1.JSONSchemaProps{
														"cluster": {
															Description: `VirtualMachineImportOvirtSourceVMClusterSpec defines the source cluster's identity of the VM in oVirt`,
															Type:        "object",
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
									Type: "string",
								},
							},
							Required: []string{"providerCredentialsSecret", "source"},
						},
						"status": {
							Type:        "object",
							Description: `VirtualMachineImportStatus defines the observed state of VirtualMachineImport`,
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"conditions": {
									Description: "A list of current conditions of the VirtualMachineImport resource",
									Type:        "array",
									Items: &extv1beta1.JSONSchemaPropsOrArray{
										Schema: &extv1beta1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
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
									Items: &extv1beta1.JSONSchemaPropsOrArray{
										Schema: &extv1beta1.JSONSchemaProps{
											Type: "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
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
	}
}

// CreateResourceMapping creates the ResourceMapping CRD
func CreateResourceMapping() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "resourcemappings.v2v.kubevirt.io",
			Labels: map[string]string{
				"operator.v2v.kubevirt.io": "",
			},
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group:   "v2v.kubevirt.io",
			Version: "v1alpha1",
			Scope:   "Namespaced",
			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
				},
			},
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:     "ResourceMapping",
				ListKind: "ResourceMappingList",
				Plural:   "resourcemappings",
				Singular: "resourcemapping",
				Categories: []string{
					"all",
				},
			},
			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]extv1beta1.JSONSchemaProps{
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
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"ovirt": {
									Type:        "object",
									Description: "OvirtMappings defines the mappings of ovirt resources to kubevirt",
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"networkMappings": {
											Type: "array",
											Description: `NetworkMappings defines the mapping of vnic profile
to network attachment definition When providing source network by name, the format is 'network name/vnic profile name'.
When providing source network by ID, the ID represents the vnic profile ID. A logical network from ovirt can be mapped
to multiple network attachment definitions on kubevirt by using vnic profile to network attachment definition mapping.`,
											Items: &extv1beta1.JSONSchemaPropsOrArray{
												Schema: &extv1beta1.JSONSchemaProps{
													Type:        "object",
													Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
													Properties: map[string]extv1beta1.JSONSchemaProps{
														"source": {
															Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
															Type:        "object",
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
											Items: &extv1beta1.JSONSchemaPropsOrArray{
												Schema: &extv1beta1.JSONSchemaProps{
													Type:        "object",
													Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
													Properties: map[string]extv1beta1.JSONSchemaProps{
														"source": {
															Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
															Type:        "object",
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
											Items: &extv1beta1.JSONSchemaPropsOrArray{
												Schema: &extv1beta1.JSONSchemaProps{
													Type:        "object",
													Description: `ResourceMappingItem defines the mapping of a single resource from the provider to kubevirt`,
													Properties: map[string]extv1beta1.JSONSchemaProps{
														"source": {
															Description: `Source defines how to identify a resource on the provider, either by ID or by name`,
															Type:        "object",
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
															Properties: map[string]extv1beta1.JSONSchemaProps{
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
	}
}

// NewClusterServiceVersion creates all cluster resources fr a specific group/component
func NewClusterServiceVersion(data *ClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {
	deployment := createOperatorDeployment(
		operatorName,
		data.Namespace,
		data.OperatorImage,
		data.ImagePullPolicy,
		"name",
		operatorName,
		int32(1),
	)

	strategySpec := csvStrategySpec{
		ClusterPermissions: []csvPermissions{
			{
				ServiceAccountName: serviceAccountName,
				Rules:              getClusterPolicyRules(),
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
					URL:  "https://github.com/kubevirt/vm-import-operator/",
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
						Name:        "virtualmachineimports.v2v.kubevirt.io",
						Version:     "v1alpha1",
						Kind:        "VirtualMachineImport",
						DisplayName: "Virtual Machine import",
						Description: "Represents a virtual machine import",
						Resources: []csvv1.APIResourceReference{
							{
								Kind:    "ConfigMap",
								Name:    "vmimport-os-mapper",
								Version: "v1",
							},
						},
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

// withOperatorLabels aggregates common lables
func withOperatorLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	for k, v := range operatorLabels {
		_, ok := labels[k]
		if !ok {
			labels[k] = v
		}
	}

	return labels
}
