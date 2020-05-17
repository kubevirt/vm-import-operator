package controller

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	resources "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operator Resource Suite")
}

var testenv *envtest.Environment
var cfg *rest.Config

var crd = resources.CreateVMImportConfig()

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(logf.ZapLoggerTo(GinkgoWriter, true))

	env := &envtest.Environment{}

	var err error
	cfg, err = env.Start()
	Expect(err).NotTo(HaveOccurred())

	opts := envtest.CRDInstallOptions{
		CRDs: []*extv1beta1.CustomResourceDefinition{crd},
	}

	crds, err := envtest.InstallCRDs(cfg, opts)
	Expect(err).NotTo(HaveOccurred())
	err = envtest.WaitForCRDs(cfg, crds, envtest.CRDInstallOptions{})
	Expect(err).NotTo(HaveOccurred())

	metrics.DefaultBindAddress = "0"

	testenv = env

	close(done)
}, 360)

var _ = AfterSuite(func() {
	if testenv == nil {
		return
	}

	testenv.Stop()

	metrics.DefaultBindAddress = ":8080"
})
