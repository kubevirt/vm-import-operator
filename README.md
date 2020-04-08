vm-import-operator
==================

Operator which imports a VM from oVirt to KubeVirt.

# Installation
```bash
kubectl create -f deploy/crds/v2v_v1alpha1_resourcemapping_crd.yaml
kubectl create -f deploy/crds/v2v_v1alpha1_virtualmachineimport_crd.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/config_map.yaml
kubectl create -f deploy/operator.yaml
```

# Import virtual machine from oVirt
## Create a secret with oVirt credentials:
```bash
cat <<EOF | kubectl create -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret-with-ovirt-credentials
type: Opaque
stringData:
  ovirt: |
    apiUrl: "https://ovirt-engine.example.com:8443/ovirt-engine/api"
    username: admin@internal # provided in the format of username@domain
    password: 123456
    caCert: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
EOF
```

The CA Certificate can be obtained from oVirt according to the instructions specified [here](http://ovirt.github.io/ovirt-engine-api-model/4.4/#_obtaining_the_ca_certificate).

## Create oVirt resource mappings that defines how oVirt resources are mapped to kubevirt:
```bash
cat <<EOF | kubectl create -f -
apiVersion: v2v.kubevirt.io/v1alpha1
kind: ResourceMapping
metadata:
  name: example-resourcemappings
  namespace: example-ns
spec:
  ovirt:
    networkMappings:
      - source:
          name: ovirtmgmt
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

## Create VM IMport resource for importing a specific VM from oVirt to KubeVirt:
```bash
cat <<EOF | kubectl create -f -
apiVersion: v2v.kubevirt.io/v1alpha1
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

An example of a VM Import resource during creation:
```
apiVersion: v2v.kubevirt.io/v1alpha1
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


# Troubleshooting

## Check operator pod for errors:
```bash
kubectl logs import-vm-oprator-xyz
```

# Development
### After cloning the repository, run the operator locally using:
```bash
export GO111MODULE=on
go mod vendor
operator-sdk run --local --namespace=default
```

### After applying changes to types file run:
```bash
operator-sdk generate k8s
```

### In order to debug the operator locally using 'dlv', start the operator locally:
Kubernetes cluster should be available and pointed by `~/.kube/config` or by `$KUBECONFIG`
The CRDs of `./deploy/crds/` should be applied on it.

```bash
operator-sdk build quay.io/$USER/vm-import-operator:v0.0.1
operator-sdk run --local --enable-delve
```
Connect to the debug session, i.e. if using vscode, create launch.json as:

```yaml
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Connect to vm-import-operator",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd/manager/main.go",
            "env": {
                "WATCH_NAMESPACE": "default",
                "KUBECONFIG": "PATH_TO_KUBECONFIG",
              },
            "args": []
        }
    ]
}
```
