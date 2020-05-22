# Introduction

VM Import operator requires the following components to be installed:

* **Kubernetes** is a container orchestration system, and is used to run
  containers on a cluster. In order to enhance full feature set of vm-import-operator,
  it is required to use OKD / OpenShift.
* **containerized-data-importer (CDI)** is an add-on which solves the problem of
  populating Kubernetes Persistent Volumes with data. It was written to be
  general purpose but with the virtualization use case in mind. Therefore, it
  has a close relationship and special integration with KubeVirt. Minimal CDI version
  is v1.15.0 in which oVirt imageio support was added.
* **KubeVirt** is an add-on which is installed on-top of Kubernetes, to be able
  to add basic virtualization functionality to Kubernetes.
* **common-templates** is a set of templates used to create KubeVirt VMs.
* **cluster-networks-addon** is an operator that deploys additional networking components
  on top of Kubernetes/OpenShift cluster. It allows to connect a VM to multiple networks,
  managing MAC Addresses pool for the VMs and more.

VM Import operator relies on templates to create best-match VM specification.
However, templates aren't k8s nor kubevirt resources, but are part of OpenShift API,
therefore as a base provider for development should be OKD / OpenShift to allow
enhancements of templates.

## Contributing to VM Import Operator

VM Import Operator contains two major components, each being a separate image:
* **vm-import-controller** is responsible for managing the VM import process, for creating a
  virtual machine and copying its disks from the source provide to k8s cluster.
* **vm-import-operator** is responsible for installing and managing the vm-import-controller
  lifecycle on k8s: upgrade, downgrade or uninstall.

### Our workflow

Contributing to VM Import operator should be as simple as possible.
Have a question? Want to discuss something? Want to contribute something?
Just open an [Issue](https://github.com/kubevirt/vm-import-operator/issues) or
a [Pull Request](https://github.com/kubevirt/vm-import-operator/pulls).

If you spot a bug or want to change something pretty simple, just go
ahead and open an Issue and/or a Pull Request, including your changes
at [kubevirt/vm-import-operator](https://github.com/kubevirt/vm-import-operator).

For bigger changes, please create a tracker Issue, describing what you want to
do. Then either as the first commit in a Pull Request, or as an independent
Pull Request, provide an **informal** design proposal of your intended changes.
The location for such proposal is [/docs/design.md](docs/design.md) in the VM Import Operator
repository. Make sure that all your Pull Requests link back to the relevant Issues.

### Getting started

Starting a cluster with the required components is described in [developers guide](docs/developeres.md)

### Testing

**Untested features do not exist**. To ensure that what we code really works,
relevant flows should be covered via unit tests and functional tests. So when
thinking about a contribution, also think about testability. All tests can be
run locally without the need of CI.
Have a look at the [functional testing](docs/functional-tests.md) for list of tests.

### Contributor compliance with Developer Certificate Of Origin (DCO)

We require every contributor to certify that they are legally permitted to contribute to our project.
A contributor expresses this by consciously signing their commits, and by this act expressing that
they comply with the [Developer Certificate Of Origin](https://developercertificate.org/).

A signed commit is a commit where the commit message contains the following content:

```
Signed-off-by: John Doe <jdoe@example.org>
```

This can be done by adding [`--signoff`](https://git-scm.com/docs/git-commit#Documentation/git-commit.txt---signoff) to your git command line.

### Getting your code reviewed/merged

Maintainers are here to help you enable your use-case in a reasonable amount
of time. The maintainers will try to review your code and give you productive
feedback in a reasonable amount of time. However, if you are blocked on a
review, or your Pull Request does not get the attention you think it deserves,
reach out for us via Comments in your Issues.

Maintainers are:

 * @jakub-dzon
 * @machacekondra
 * @masayag
 * @pkliczewski

## Projects & Communities

### [KubeVirt](https://github.com/kubevirt/)

* Getting started
  * [Developer Guide](https://github.com/kubevirt/kubevirt/tree/master/docs/getting-started.md)
  * [Demo](https://github.com/kubevirt/demo)
  * [Documentation](https://github.com/kubevirt/kubevirt/tree/master/docs/)

### [Kubernetes](http://kubernetes.io/)

* Getting started
  * [http://kubernetesbyexample.com](http://kubernetesbyexample.com)
  * [Hello Minikube - Kubernetes](https://kubernetes.io/docs/tutorials/stateless-application/hello-minikube/)
  * [User Guide - Kubernetes](https://kubernetes.io/docs/user-guide/)
* Details
  * [Declarative Management of Kubernetes Objects Using Configuration Files - Kubernetes](https://kubernetes.io/docs/concepts/tools/kubectl/object-management-using-declarative-config/)
  * [Kubernetes Architecture](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/architecture.md)

## Additional Topics

* Golang
  * [Documentation - The Go Programming Language](https://golang.org/doc/)
  * [Getting Started - The Go Programming Language](https://golang.org/doc/install)
* Patterns
  * [Introducing Operators: Putting Operational Knowledge into Software](https://coreos.com/blog/introducing-operators.html)
  * [Microservices](https://martinfowler.com/articles/microservices.html) nice
    content by Martin Fowler
* Testing
  * [Ginkgo - A Golang BDD Testing Framework](https://onsi.github.io/ginkgo/)
