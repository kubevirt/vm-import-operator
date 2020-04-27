# Virtual Machine Import operator design

## Introduction

VM Import operator is responsible for importing virtual machines that originated in an external virtual system into a kubevirt cluster. The current proposal refers to importing a single VM from [oVirt](https://www.ovirt.org/) to kubevirt. However it should remain open to support additional sources (e.g. vmware/OVA).

A [repository](https://github.com/kubevirt/vm-import-operator) was created with a proposal for the vm-import resource and additional required resources to enable access to the source oVirt provider, a custom resource that specifies the mapping of oVirt resources to KubeVirt resources and the custom resource that defines the VM Import.

Contact Information: Moti Asayag ([masayag@redhat.com](mailto:masayag@redhat.com))

## API Design

The resources for initiating the VM Import process are:
1. VirtualMachineImport resource that defines the process: source provider, source VM and mappings.
2. ResourceMapping resource that defines the resource mappings from source provider to kubevirt (optional)
3. A secret that defines the endpoint and credentials to the source provider

Each will be described in details:

### VirtualMachineImport

VirtualMachineImport is a namespaced custom resource that defines the source of the VM, the identifier of the VM on the source and the mapping to be used for the import.

An example of the [VirtualMachineImport](https://github.com/kubevirt/vm-import-operator/blob/master/deploy/crds/v2v_v1alpha1_virtualmachineimport_cr.yaml) resource is shown below.

```yaml
apiVersion: v2v.kubevirt.io/v1alpha1
kind: VirtualMachineImport
Metadata:
  annotations:
    vmimport.v2v.kubevirt.io/progress: 20
  labels:
    vmimport.v2v.kubevirt.io/tracker: vmimport-123
  name: example-virtualmachineimport
  namespace: example-ns
Spec:
  providerCredentialsSecret: # A secret holding the access credentials to ovirt, see example below
    name: my-secret-with-ovirt-credentials
    namespace: default # optional, if not specified, use CR's
  targetVmName: examplevm # The target name is optional. If not provided, the import will attempt to use the normalized source VM name, generated name by the template or a generated name by the provider.
  startVm: true # Indicates if the target VM should be started at the end of the import process. Default is ‘false’
  resourceMapping: # optional reference to master mapping defined in cr
    name: map-of-ovirt-resources-to-kubevirt # a mapping of ovirt resource (network, storage)
    namespace: othernamespace # optional, if not specified, use CR's namespace
  source:
    ovirt:  # represents ovirt-engine to import from the virtual machine
      vm: # in order to uniquely identify a VM on ovirt we need to provide (VM name,cluster) or use (VM id)
        id: 80554327-0569-496b-bdeb-fcbbf52b827b
        # if VM id wasn’t provided, it requires to specify name and cluster[.id|.name]
        name: myvm
        cluster:
          name: mycluster
          id: CC111111-1111-1111-1111-111111111111
      mappings:
        networkMappings:
        - source:
            name: red
            target: xyz
            type: bridge
        diskMappings: # a mapping of a specific disk to storage class
        - source:
            id: 8181ecc1-5db8-4193-9c92-3ddab3be7b12
            target: local-storage
status:
 targetVmName: myvm # the name of the created virtual machine
 conditions:
   - lastHeartbeatTime: "2020-04-22T16:59:16Z"
      lastTransitionTime: "2020-04-22T16:59:14Z"
      message: Copying virtual machine disks
      reason: CopyingDisks
      status: "True"
      type: Processing  # indicates if the VM import process is running
 dataVolumes: # list of data volumes created for the VMs by the operator
   - name: 8181ecc1-5db8-4193-9c92-3ddab3be7b05
   - name: 8877699c-5c62-4b55-969b-45f8e07c25e9
```

The “source” element can be extended to represent additional source types for the VM import resource, such as VMWare and OVA, to which a tailored resource mapping will be required.

### Progress monitoring

Monitoring the progress of the vm import will be done by updating an annotation *vmimport.v2v.kubevirt.io/progress* with a numeric value in scale of 1 to 100. It is up to the operator to update the progress, in each step that is achieved, e.g:
* progressStart        = "0"
* progressCreatingVM   = "30"
* progressCopyingDisks = "40"
* progressStartVM      = "90"
* progressDone         = "100"

### Resource Mappings

The mapping between ovirt to kubevirt resources is defined in ResourceMapping custom resource. The CR will contain sections for the mapping resources: network and storage. The example below demonstrates how multiple entities of each resource type can be declared and mapped.

Each section specifies the mapping between ovirt entity to kubevirt’s entity, and allows to provide an additional piece of information, e.g. interface type if needed to complete the VM spec.

The import VM operator will be responsible to deduce the configuration of the target VM based on the configuration of the source and perform the transformation in a way that will preserve the attributes of the source VM, e.g. boot sequence, MAC address, [run strategy](https://kubevirt.io/user-guide/docs/latest/creating-virtual-machines/run-strategies.html), [domain specification](https://kubevirt.io/api-reference/v0.26.1/definitions.html#_v1_domainspec) and hostname if available on ovirt by the guest agent.

```yaml
apiVersion: v2v.kubevirt.io/v1alpha1
kind: ResourceMapping
metadata:
 name: example-ovirtresourcemappings
 namespace: example-ns
Spec:
  ovirt:
    networkMappings:
    - source:
        name: red # maps of ovirt logic network to network attachment definition
      target: xyz
      type: bridge
    - source:
        name: ovirtmgmt
      Target:
        name: pod
      type: pod
    storageMappings:
    - source:
        name: ovirt_storage_domain_1 # maps ovirt storage domains to storage class
      target: storage_class_1
```

### Provider Secret

The [example](https://github.com/kubevirt/vm-import-operator/blob/master/examples/ovirt-secret.yaml) of secret below defines oVirt connectivity and authentication method:

```yaml
apiVersion: v1
kind: Secret
metadata:
 name: my-secret-with-ovirt-credentials
type: Opaque
stringData:
 ovirt: |-
   apiUrl: "https://my.ovirt-engine-server/ovirt-engine/api/"
   username: admin@internal # provided in the format of username@domain
                            # the user should have enough permissions on ovirt side to stop the VM
   password: 123456
   ca.cert: | # The certificate presented by the server will be verified using these CA certificates.
              # If not set, system wide CA certificate store is used. Expected in base64 format.
   -----BEGIN CERTIFICATE-----
...
   -----END CERTIFICATE-----
```

### Import Validations

Due to the fact that oVirt provides a wider set of features that aren’t supported by kubevirt, the target VM might be created differently than the source VM configuration. That requires to warn the user or to block the import process.

The admission rules will be split into three categories: log, warn and block:
* Log - a validation rule that cannot map ovirt behavior to kubevirt, however, it is harmless. In that case, that violation will be logged. E.g.. nic_boot set to false on source VM, a logical name set to a disk on the source, Rng_device other than urandom and more.
* Warn - a validation rule that might introduce an issue. The violation will be recorded to the status of the CR, letting the user decide if the import should be cancelled. E.g., vm nic was unplugged on ovirt. In that case, the interface is not added to the target VM.
* Block - a validation that fails the import action if violated. In this case, the import is failed. E.g., a missing mapping entry.

The entire list of import validation rules is [here](rules.md) (created by Jakub Dzon).