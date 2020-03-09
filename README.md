vm-import-operator
==================

Operator which imports a VM from oVirt to KubeVirt.

# Installation
```bash
kubectl create -f deploy/crds/v2v_v1alpha1_virtualmachineimport_crd.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
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
  config.yaml: |-
    apiUrl: "https://ovirt-engine.example.com:8443/ovirt-engine/api/"
    username: admin@internal # provided in the format of username@domain
    password: 123456
    insecure: true
EOF
```

## Create a config map that defines the resource mappings from oVirt to KubeVirt:
```bash
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: ovirt-mapping-example
  namespace: default
data:
  mappings: |-
    networkMapping:
      - source: red # maps of ovirt logic network to network attachment definition
        target: xyz
        type: bridge
      - source: ovirtmgmt
        target: pod
        type: pod
    storageMapping:
      - source: ovirt_storage_domain_1 # maps ovirt storage domains to storage class
        target: storage_class_1
    affinityMapping: # affinity mappings will be limited at first for 'pinned-to-host', since affinity/anti-affinity requires more than a single vm import
      - source: ovirt_node_1
        target: k8s_node_1
        policy: nodeAffinity
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
  source:
    ovirt:  # represents ovirt-engine to import from the virtual machine
      providerCredentialsSecret: # A secret holding the access credentials to ovirt, see example ovirt-mapping-example.yaml
        name: my-secret-with-ovirt-credentials
        namespace: default # optional, if not specified, use CR's namespace
      vm: # in order to uniquely identify vm on ovirt with need to provide (vm_name,cluster) or use (vm-id)
        id: 80554327-0569-496b-bdeb-fcbbf52b827b
  resourceMappings:
    configMapName: ovirt-mapping-example # a mapping of ovirt resource (network, storage, affinity)
    configMapNamespace: default # optional, if not specified, use CR's namespace
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
  name: example-virtualmachineimport
  namespace: default
spec:
  source:
  ...
status:
  targetVirtualMachineName: myvm # the name of the created virtual machine
  conditions:
    - lastProbeTime: null
      lastTransitionTime: "2020-02-20T12:43:10Z"
      status: "False"
      type: Ready # indicates if the vm import process is completed or not
      message: VM import is in progress.
    - lastProbeTime: null
      lastTransitionTime: "2020-02-20T12:43:10Z"
      status: "True"
      type: Processing # indicates if the vm import process is running
      reason: CopyingDisk
      message: VM disks are being copied to destination.
  state: Running
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
operator-sdk up local --namespace=default
```

### After applying changes to types file run:
```bash
operator-sdk generate k8s
```

### In order to debug the operator locally using 'dlv', start the operator locally:
```bash
operator-sdk build quay.io/$USER/vm-import-operator:v0.0.1
OPERATOR_NAME=import-vm-operator WATCH_NAMESPACE=default ./build/_output/bin/import-vm-operator
```

Kubernetes cluster should be available and pointed by `~/.kube/config`.
The CRDs of `./deploy/crds/` should be applied on it.

From a second terminal window run:
```bash
dlv attach --headless --api-version=2 --listen=:2345 $(pgrep -f vm-import-operator) ./build/_output/bin/vm-import-operator
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
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 2345,
            "host": "127.0.0.1",
            "program": "${workspaceFolder}",
            "env": {},
            "args": []
        }
    ]
}
```