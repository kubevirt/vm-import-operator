package controller

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"k8s.io/apimachinery/pkg/runtime"

	resources "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operator Resource Suite")
}

var testenv *envtest.Environment
var cfg *rest.Config

var crd = resources.CreateVMImportConfig()

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(false), zap.WriteTo(GinkgoWriter)))

	env := &envtest.Environment{}

	var err error
	cfg, err = env.Start()
	Expect(err).NotTo(HaveOccurred())

	opts := envtest.CRDInstallOptions{
		CRDs: []runtime.Object{crd},
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
