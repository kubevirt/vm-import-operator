# Developer information

# Creating k8s cluster

One alternative to create a running cluster fit for development is using [CodeReady Containers](https://code-ready.github.io/crc/)

### Start CRC
Increase the default memory for a smooth run of the cluster:
```bash
crc config set memory 10000
crc setup
crc start
```
Login into CodeReady cluster as admin according to the message printed to the console.
Alternately, you can set KUBECONFING environment variable to point to the created file.
A possible location of kubeconfig file is `$HOME/.crc/cache/crc_libvirt_$OCP_VERSION/kubeconfig`

### Use HCO to deploy components
Use hyperconverged-cluster-operator (HCO) for installing required components:
```bash
git clone https://github.com/kubevirt/hyperconverged-cluster-operator.git
cd hyperconverged-cluster-operator
make
./hack/deploy.sh
```
### Create local storage class
```bash
cat <<EOF | kubectl create -f -
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: local-storage
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
EOF

# Patch all PVs to use storage class
for f in $(oc get pv -o name)
do
  oc patch $f -p '{"spec":{"storageClassName":"local-storage"}}'
done
```

At this point the cluster should be ready for deploying the vm-import-operator.
Made required changes to the operator/controller, build, run tests and deploy:
```bash
make
# if needed, provide a specific tag by using IMAGE_TAG
IMAGE_TAG=YOUR_TAG make docker-build docker-push

# generated manifests that matches version and quay user
IMAGE_TAG=YOUR_TAG VERSION=YOUR_TAG QUAY_USER=$USER make gen-manifests

# deploy the manifests
kubectl apply -f ./manifests/vm-import-operator/YOUR_TAG/operator.yaml
kubectl apply -f ./manifests/vm-import-operator/YOUR_TAG/vmimportconfig_cr.yaml
```
In order to speed-up development cycle of vm-import-controller, use this method instead:
```
./hack/generate-controller-manifests.sh
kubectl apply -f ./build/_output/deploy/vm-import-controller-local-manifests.yaml
CGO_ENABLED=0 GOOS=linux go build -o build/_output/bin/vm-import-controller-local cmd/manager/main.go
DEV_LOCAL_DEBUG="true" WATCH_NAMESPACE="" build/_output/bin/vm-import-controller-local
```
At this step you can submit your VMImport custom-resource with the rules to import a VM from its source provider.

### Remote Debugging of vm-import-controller
```bash
kubectl apply -f `./hack/generate-controller-manifests.sh`
make debug-controller
```
Connect to the debug session, i.e. if using vscode, create launch.json as:

```yaml
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Connect to vm-import-controller",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd/manager/main.go",
            "env": {
                "WATCH_NAMESPACE": "",
                "KUBECONFIG": "PATH_TO_KUBECONFIG",
                "DEV_LOCAL_DEBUG": "true",
                "OPERATOR_NAME": "vm-import-controller"
              },
            "args": []
        }
    ]
}
```
### Remote Debugging of vm-import-operator
```bash
make debug-operator
```
Connect to the debug session, i.e. if using vscode, create launch.json as (update variables as needed):

```yaml
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Connect to vm-import-operator",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "port": 2346,
            "program": "${workspaceFolder}/cmd/operator/operator.go",
            "env": {
                "WATCH_NAMESPACE": "",
                "KUBECONFIG": "PATH_TO_KUBECONFIG",
                "DEV_LOCAL_DEBUG": "true",
                "OPERATOR_NAME": "vm-import-operator",
                "DEPLOY_CLUSTER_RESOURCES": "true",
                "OPERATOR_VERSION": "v0.0.2",
                "CONTROLLER_IMAGE": "quay.io/kubevirt/vm-import-controller:v0.0.2",
                "PULL_POLICY": "Always"
              },
            "args": []
        }
    ]
}
```

# Functional testing

Functional tests for the operator are described in a [this document](functional-tests.md).
Run functional tests simply by:
```bash
./automation/test.sh
```

### Running tests

In order to run controller tests we need to install `kubebuilder` by running:

```bash
os=$(go env GOOS)
arch=$(go env GOARCH)

# download kubebuilder and extract it to tmp
curl -L https://go.kubebuilder.io/dl/2.3.1/${os}/${arch} | tar -xz -C /tmp/

# move to a long-term location and put it on your path
sudo mv /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder

```

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

# Troubleshooting

## Check operator pod for errors:
For errors related to the vm-import-operator, check logs of:
```bash
kubectl logs vm-import-operator-xyz
```
Also check the status of the VMImportConfig resource:
```bash
kubectl describe vmimportconfig vm-import-operator-config
```

## Check controller pod for errors:
For errors related to the vm-import-controller, check logs of:
```bash
kubectl logs vm-import-controller-xyz
```
Also check the status of the VMImport resource:
```bash
kubectl describe vmimport RESOURCE_NAME
```