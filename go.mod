module github.com/kubevirt/vm-import-operator

go 1.13

require (
	cloud.google.com/go v0.53.0 // indirect
	github.com/RHsyseng/operator-utils v0.0.0-20190906175225-942a3f9c85a9
	github.com/aktau/github-release v0.7.2
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.4
	github.com/google/go-cmp v0.4.1 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20191119172530-79f836b90111
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/machacekondra/fakeovirt v0.0.0-20200617055337-1afdfa789aab
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/client-go v0.0.0
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190725173916-b56e63a643cc
	github.com/operator-framework/operator-sdk v0.18.0
	github.com/ovirt/go-ovirt v4.3.4+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1
	github.com/spf13/pflag v1.0.5
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80 // indirect
	github.com/vmware/govmomi v0.23.0
	github.com/voxelbrain/goptions v0.0.0-20180630082107-58cddc247ea2 // indirect
	golang.org/x/tools v0.0.0-20200403190813-44a64ad78b9b
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.18.6
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	kubevirt.io/client-go v0.26.2
	kubevirt.io/containerized-data-importer v1.22.0
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/ovirt/go-ovirt => github.com/ovirt/go-ovirt v0.0.0-20200428093010-9bcc4fd4e6c0

replace (
	k8s.io/api => k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.6
	k8s.io/apiserver => k8s.io/apiserver v0.18.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.6
	k8s.io/client-go => k8s.io/client-go v0.18.6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.6
	k8s.io/code-generator => k8s.io/code-generator v0.18.6
	k8s.io/component-base => k8s.io/component-base v0.18.6
	k8s.io/cri-api => k8s.io/cri-api v0.18.6
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.6
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.6
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.6
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.6
	k8s.io/kubectl => k8s.io/kubectl v0.18.6
	k8s.io/kubelet => k8s.io/kubelet v0.18.6
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.6
	k8s.io/metrics => k8s.io/metrics v0.18.6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.6
)

replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20200526144822-34f54f12813a

replace sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.0

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by O

replace vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
