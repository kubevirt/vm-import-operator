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
TARGET_NAMESPACE=YOUR_DESIRED_NAMESPACE make gen-manifests
```

Deploy vm-import-operator resources:
```bash
kubectl apply -f https://github.com/kubevirt/vm-import-operator/releases/download/v0.0.1/v2v_v1alpha1_resourcemapping_crd.yaml
kubectl apply -f https://github.com/kubevirt/vm-import-operator/releases/download/v0.0.1/v2v_v1alpha1_virtualmachineimport_crd.yaml
# TODO: remove config_map.yaml once v0.0.2 is released
kubectl apply -f https://github.com/kubevirt/vm-import-operator/releases/download/v0.0.1/config_map.yaml
kubectl apply -f https://github.com/kubevirt/vm-import-operator/releases/download/v0.0.1/operator.yaml
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
### After cloning the repository, deploy vm-import-operator resources:
```bash
kubectl apply -f manifests/vm-import-operator/v0.0.1/v2v_v1alpha1_resourcemapping_crd.yaml
kubectl apply -f manifests/vm-import-operator/v0.0.1/v2v_v1alpha1_virtualmachineimport_crd.yaml
# TODO: remove config_map.yaml once v0.0.2 is released
kubectl apply -f manifests/vm-import-operator/v0.0.1/config_map.yaml
kubectl apply -f manifests/vm-import-operator/v0.0.1/operator.yaml
# since operator.yaml deploys the operator, remove it to use local one
kubectl delete deployment -n kubevirt-hyperconverged vm-import-operator
```
### Run the operator locally using:
```bash
make vendor
operator-sdk run --local --namespace=default
```

### After applying changes to file run:
```bash
make
```

### In order to debug the operator locally using 'dlv', start the operator locally:
Kubernetes cluster should be available and pointed by `~/.kube/config` or by `$KUBECONFIG`

```bash
make docker-build
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

# Running tests

In order to run controller tests we need to install `kubebuilder` by running:

```bash
os=$(go env GOOS)
arch=$(go env GOARCH)

# download kubebuilder and extract it to tmp
curl -L https://go.kubebuilder.io/dl/2.3.1/${os}/${arch} | tar -xz -C /tmp/

# move to a long-term location and put it on your path
sudo mv /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder

```

# Functional testing
Functional tests for the operator are described in a [this document](docs/functional-tests.md).

# Release
1. Checkout a public branch
2. Call `make prepare-patch|minor|major` and prepare release notes
3. Open a new PR
4. Once the PR is merged, create a new release in GitHub and attach new manifests

## Releasing as a draft release
For testing/development purposes, a developer may create a draft release on developer's github repository,
by providing the desired set of parameters:

```bash
make prepare-patch
GITHUB_REPOSITORY=origin \
GITHUB_TOKEN=1eb... \
GITHUB_USER=$USER \
EXTRA_RELEASE_ARGS=--draft \
make release
```
EXTRA_RELEASE_ARGS will be passed as-is to `github-release` (other values maybe '--pre-release'). See manual pages for further information.