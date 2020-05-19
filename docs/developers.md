# Developer information

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
### After applying changes to file run:
```bash
make
```

### In order to debug the operator or the controller locally using 'dlv', start the operator locally:
Kubernetes cluster should be available and pointed by `~/.kube/config` or by `$KUBECONFIG`

#### Remote Debugging of vm-import-controller
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
#### Remote Debugging of vm-import-operator
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
```bash
kubectl logs import-vm-oprator-xyz
```
