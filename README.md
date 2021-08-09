vm-import-operator
==================

Operator which imports a VM from oVirt to KubeVirt.

# Designs
* [Operator and custom resource design](docs/design.md)
* [Virtual machine import rules](docs/rules.md)

# Installation
Installation has to be made into KubeVirt installation namespace.
Default kubevirt namespace is assumed to be 'kubevirt-hyperconverged'.
In order to generate manifests for a different namespace run:
```bash
export TARGET_NAMESPACE=YOUR_DESIRED_NAMESPACE
export VERSION=YOUR_DESIRED_VERSION
make gen-manifests
```

Deploy vm-import-operator resources:
```bash
kubectl apply -f https://github.com/kubevirt/vm-import-operator/releases/download/v0.4.0/operator.yaml
kubectl apply -f https://github.com/kubevirt/vm-import-operator/releases/download/v0.4.0/vmimportconfig_cr.yaml
```

In order to import you will be required to have:
* [Kubevirt](https://github.com/kubevirt/kubevirt)
* [Containerized Data Importer](https://github.com/kubevirt/containerized-data-importer)
* [Common Templates](https://github.com/kubevirt/common-templates)

Optional for network configurations:
* [Cluster Networks Addon](https://github.com/kubevirt/cluster-network-addons-operator)

# Import virtual machine from oVirt
## Create a secret with oVirt credentials:
```bash
cat <<EOF | kubectl create -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret-with-ovirt-credentials
  namespace: default
type: Opaque
stringData:
  ovirt: |
    apiUrl: "https://ovirt-engine.example.com:8443/ovirt-engine/api"
    username: admin@internal # provided in the format of username@domain
    password: 123456
    caCert: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
EOF
```

The CA Certificate can be obtained from oVirt according to the instructions specified [here](http://ovirt.github.io/ovirt-engine-api-model/4.4/#_obtaining_the_ca_certificate).

## Create oVirt resource mappings that defines how oVirt resources are mapped to kubevirt:
```bash
cat <<EOF | kubectl create -f -
apiVersion: v2v.kubevirt.io/v1beta1
kind: ResourceMapping
metadata:
  name: example-resourcemappings
  namespace: example-ns
spec:
  ovirt:
    networkMappings:
      - source:
          name: ovirtmgmt/ovirtmgmt
        target:
          name: pod
        type: pod
    storageMappings:
      - source:
          name: ovirt_storage_domain_1
        target:
          name: storage_class_1
EOF
```

## Create VM Import resource for importing a specific VM from oVirt to KubeVirt:
```bash
cat <<EOF | kubectl create -f -
apiVersion: v2v.kubevirt.io/v1beta1
kind: VirtualMachineImport
metadata:
  name: example-virtualmachineimport
  namespace: default
spec:
  providerCredentialsSecret:
    name: my-secret-with-ovirt-credentials
    namespace: default # optional, if not specified, use CR's namespace
  resourceMapping:
    name: example-resourcemappings
    namespace: othernamespace
  targetVmName: testvm
  startVm: false
  source:
    ovirt:
      vm:
        id: 80554327-0569-496b-bdeb-fcbbf52b827b
EOF
```

## Verify VM Import resource was created:

```bash
kubectl get vmimports example-virtualmachineimport -n default
```

### Wait for VM Import resource to complete:
```bash
kubectl wait vmimports example-virtualmachineimport -n default --for=condition=Succeeded
```

### View vm-import logs:
```bash
kubectl logs -n kubevirt-hyperconverged deploy/vm-import-controller
```

An example of a VM Import resource during creation:
```
apiVersion: v2v.kubevirt.io/v1beta1
kind: VirtualMachineImport
metadata:
  annotations:
    vmimport.v2v.kubevirt.io/progress: 20
  name: example-virtualmachineimport
  namespace: default
spec:
  source:
  ...
status:
  targetVmName: myvm
  conditions:
    - lastTransitionTime: "2020-02-20T12:43:10Z"
      status: "False"
      type: Succeeded
      reason: DataVolumeCreationFailed
      message: VM import due to failure to copy source VM disks to kubevirt.
    - lastTransitionTime: "2020-02-20T12:43:10Z"
      status: "False"
      type: Processing
      reason: CopyingDisk
      message: VM disks are being copied to destination.
  dataVolumes: # list of data volumes created for the VMs
    - name: dv-myvm-1
    - name: dv-myvm-2
```

# How to contribute

To learn how to contribute, please refer to [contribution guide](CONTRIBUTING.md).
