module github.com/kubevirt/vm-import-operator

go 1.13

require (
	contrib.go.opencensus.io/exporter/ocagent v0.4.11 // indirect
	github.com/Masterminds/sprig v0.0.0-20190301161902-9f8fceff796f // indirect
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/go-openapi/spec v0.19.4
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.7.1
	github.com/operator-framework/operator-sdk v0.15.2
	github.com/ovirt/go-ovirt v4.3.4+incompatible
	github.com/pelletier/go-toml v1.3.0 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/technosophos/moniker v0.0.0-20180509230615-a5dbd03a2245 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	golang.org/x/build v0.0.0-20190314133821-5284462c4bec // indirect
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.17.1
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	kubevirt.io/client-go v0.26.2
	kubevirt.io/containerized-data-importer v1.13.0
	sigs.k8s.io/controller-runtime v0.4.0
	vbom.ml/util v0.0.0-20180919145318-efcd4e0f9787 // indirect
)

replace github.com/ovirt/go-ovirt => github.com/ovirt/go-ovirt v0.0.0-20200320082526-4e97a11ff083

replace kubevirt.io/containerized-data-importer => github.com/pkliczewski/containerized-data-importer v1.10.2-0.20200302082837-f4b489dba1bc

// Pinned to kubernetes-1.16.2
replace (
	k8s.io/api => k8s.io/api v0.0.0-20191016110408-35e52d86657a
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191016112112-5190913f932d
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191016114015-74ad18325ed5
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191016115326-20453efc2458
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191016115129-c07a134afb42
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191016111319-039242c015a9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191016115521-756ffa5af0bd
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191016112429-9587704a8ad4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191016114939-2b2b218dc1df
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191016114407-2e83b6f20229
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191016114748-65049c67a58b
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191016114556-7841ed97f1b2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191016115753-cf0698c3a16b
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191016113814-3b1a734dba6e
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191016112829-06bb3c9d77c9
)

replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20191125132246-f6563a70e19a

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved

replace sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
