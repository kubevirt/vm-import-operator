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
