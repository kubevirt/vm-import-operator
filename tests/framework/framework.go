package framework

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	sclient "github.com/machacekondra/fakeovirt/pkg/client"

	"github.com/onsi/gomega"
	"k8s.io/klog"

	vmiclientset "github.com/kubevirt/vm-import-operator/pkg/api-client/clientset/versioned"
	"github.com/onsi/ginkgo"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"kubevirt.io/client-go/kubecli"
	cdi "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

const (
	nsCreateTime = 60 * time.Second
	nsDeleteTime = 5 * time.Minute
	//NsPrefixLabel provides a virtual machine import prefix label to identify the test namespace
	NsPrefixLabel = "vm-import-e2e"
)

// run-time flags
var (
	kubectlPath              *string
	kubeConfig               *string
	master                   *string
	ovirtCAPath              *string
	kubeVirtInstallNamespace *string
	defaultStorageClass      *string
	nfsStorageClass          *string
	imageioInstallNamespace  *string
)

// This package is based on https://github.com/kubevirt/containerized-data-importer/blob/master/tests/framework/framework.go
// Framework supports common operations used by functional/e2e tests.
type Framework struct {
	// NsPrefix is a prefix for generated namespace
	NsPrefix string
	//  k8sClient provides our k8s client pointer
	K8sClient *kubernetes.Clientset
	// VMImportClient provides Virtual Machine Import client pointer
	VMImportClient *vmiclientset.Clientset
	// KubeVirtClient provides KubeVirt client pointer
	KubeVirtClient kubecli.KubevirtClient
	// CdiClient provides our CDI client pointer
	CdiClient *cdi.Clientset
	// RestConfig provides a pointer to our REST client config.
	RestConfig *rest.Config
	// OvirtStubbingClient provides a way to stub oVirt behavior
	OvirtStubbingClient *sclient.FakeOvirtClient
	// Namespace provides a namespace for each test generated/unique ns per test
	Namespace          *v1.Namespace
	namespacesToDelete []*v1.Namespace

	// KubectlPath is a test run-time flag so we can find kubectl
	KubectlPath string
	// KubeConfig is a test run-time flag to store the location of our test setup kubeconfig
	KubeConfig string
	// Master is a test run-time flag to store the id of our master node
	Master string
	// The namespaced name of the oVirt secret to copy for tests
	OVirtCA string
	// KubeVirtInstallNamespace namespace where KubeVirt is installed
	KubeVirtInstallNamespace string

	// DefaultStorageClass specifies the name of a basic, default storage class
	DefaultStorageClass string
	// NfsStorageClass specifies the name of an NFS-based storage class
	NfsStorageClass string

	// ImageioInstallNamespace namespace where ImageIO and FakeOvirt are installed
	ImageioInstallNamespace string
}

// initialize run-time flags
func init() {
	// Make sure that go test flags are registered when the framework is created
	testing.Init()
	kubectlPath = flag.String("kubectl-path", "kubectl", "The path to the kubectl binary")
	kubeConfig = flag.String("kubeconfig", "/var/run/kubernetes/admin.kubeconfig", "The absolute path to the kubeconfig file")
	master = flag.String("master", "", "master url:port")
	ovirtCAPath = flag.String("ovirt-ca", "", "Path to the oVirt CA file")
	kubeVirtInstallNamespace = flag.String("kubevirt-namespace", "kubevirt", "Set the namespace KubeVirt is installed in")
	defaultStorageClass = flag.String("default-sc", "local", "Set the name of the default storage class")
	nfsStorageClass = flag.String("nfs-sc", "nfs", "Set the name of a NFS-based storage class")
	imageioInstallNamespace = flag.String("imageio-namespace", "cdi", "Set the namespace ImageIO and FakeOvirt are installed in")
}

// NewFrameworkOrDie calls NewFramework and handles errors by calling Fail. Config is optional, but
// if passed there can only be one.
func NewFrameworkOrDie(prefix string) *Framework {
	f, err := NewFramework(prefix)
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("failed to create test framework: %v", err))
	}
	return f
}

// NewFramework makes a new framework and sets up the global BeforeEach/AfterEach's.
// Test run-time flags are parsed and added to the Framework struct.
func NewFramework(prefix string) (*Framework, error) {
	f := &Framework{
		NsPrefix: prefix,
	}

	// handle run-time flags
	if !flag.Parsed() {
		flag.Parse()
	}

	f.KubectlPath = *kubectlPath
	f.KubeConfig = *kubeConfig
	f.Master = *master
	f.KubeVirtInstallNamespace = *kubeVirtInstallNamespace
	f.DefaultStorageClass = *defaultStorageClass
	f.NfsStorageClass = *nfsStorageClass
	f.ImageioInstallNamespace = *imageioInstallNamespace
	ovirtClient := sclient.NewInsecureFakeOvirtClient("https://localhost:12346")
	f.OvirtStubbingClient = &ovirtClient
	content, err := ioutil.ReadFile(*ovirtCAPath)
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, cannot read oVirt CA file")
	}
	f.OVirtCA = string(content)

	restConfig, err := f.LoadConfig()
	if err != nil {
		// Can't use Expect here due this being called outside of an It block, and Expect
		// requires any calls to it to be inside an It block.
		return nil, errors.Wrap(err, "ERROR, unable to load RestConfig")
	}
	f.RestConfig = restConfig
	// clients
	kcs, err := f.GetKubeClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create K8SClient")
	}
	f.K8sClient = kcs
	vmics, err := f.GetVMImportClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create VMImportClient")
	}
	f.VMImportClient = vmics
	kvClient, err := f.GetKubeVirtClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create KubeVirt client")
	}
	f.KubeVirtClient = kvClient
	cs, err := f.GetCdiClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create CdiClient")
	}
	f.CdiClient = cs

	ginkgo.BeforeEach(f.BeforeEach)
	ginkgo.AfterEach(f.AfterEach)

	return f, err
}

// BeforeEach provides a set of operations to run before each test
func (f *Framework) BeforeEach() {
	// generate unique primary ns (ns2 not created here)
	ginkgo.By(fmt.Sprintf("Building a %q namespace api object", f.NsPrefix))
	ns, err := f.CreateNamespace(f.NsPrefix, map[string]string{
		NsPrefixLabel: f.NsPrefix,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	f.Namespace = ns
	f.AddNamespaceToDelete(ns)

	err = f.OvirtStubbingClient.Reset("static-sso,static-namespace,static-transfers")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

// AfterEach provides a set of operations to run after each test
func (f *Framework) AfterEach() {
	f.CleanUp()
}

// CleanUp provides a set of operations clean the namespace
func (f *Framework) CleanUp() {
	// delete the namespace(s) in a defer in case future code added here could generate
	// an exception. For now there is only a defer.
	defer func() {
		for _, ns := range f.namespacesToDelete {
			defer func() { f.namespacesToDelete = nil }()
			if ns == nil || len(ns.Name) == 0 {
				continue
			}
			ginkgo.By(fmt.Sprintf("Destroying namespace %q for this suite.", ns.Name))
			err := DeleteNS(f.K8sClient, ns.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}
	}()
}

// CreateNamespace instantiates a new namespace object with a unique name and the passed-in label(s).
func (f *Framework) CreateNamespace(prefix string, labels map[string]string) (*v1.Namespace, error) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("vm-import-e2e-tests-%s-", prefix),
			Namespace:    "",
			Labels:       labels,
		},
		Status: v1.NamespaceStatus{},
	}

	var nsObj *v1.Namespace
	c := f.K8sClient
	err := wait.PollImmediate(2*time.Second, nsCreateTime, func() (bool, error) {
		var err error
		nsObj, err = c.CoreV1().Namespaces().Create(ns)
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil // done
		}
		klog.Warningf("Unexpected error while creating %q namespace: %v", ns.GenerateName, err)
		return false, err // keep trying
	})
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Created new namespace %q\n", nsObj.Name)
	return nsObj, nil
}

// AddNamespaceToDelete provides a wrapper around the go append function
func (f *Framework) AddNamespaceToDelete(ns *v1.Namespace) {
	f.namespacesToDelete = append(f.namespacesToDelete, ns)
}

// DeleteNS provides a function to delete the specified namespace from the test cluster
func DeleteNS(c *kubernetes.Clientset, ns string) error {
	return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
		err := c.CoreV1().Namespaces().Delete(ns, nil)
		if err != nil && !apierrs.IsNotFound(err) {
			return false, nil // keep trying
		}
		// see if ns is really deleted
		_, err = c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil // deleted, done
		}
		if err != nil {
			klog.Warningf("namespace %q Get api error: %v", ns, err)
		}
		return false, nil // keep trying
	})
}

// GetKubeClient returns a Kubernetes rest client
func (f *Framework) GetKubeClient() (*kubernetes.Clientset, error) {
	return GetKubeClientFromRESTConfig(f.RestConfig)
}

// GetVMImportClient gets an instance of a Virtual Machine Import client
func (f *Framework) GetVMImportClient() (*vmiclientset.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
	if err != nil {
		return nil, err
	}
	vmiClient, err := vmiclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return vmiClient, nil
}

// GetVMImportClient gets an instance of a KubeVirt client
func (f *Framework) GetKubeVirtClient() (kubecli.KubevirtClient, error) {
	kvClient, err := kubecli.GetKubevirtClientFromFlags(f.Master, f.KubeConfig)
	if err != nil {
		return nil, err
	}
	return kvClient, nil
}

// GetCdiClient gets an instance of a kubernetes client that includes all the CDI extensions.
func (f *Framework) GetCdiClient() (*cdi.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
	if err != nil {
		return nil, err
	}
	cdiClient, err := cdi.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cdiClient, nil
}

// LoadConfig loads our specified kubeconfig
func (f *Framework) LoadConfig() (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
}

// GetKubeClientFromRESTConfig provides a function to get a K8s client using the REST config
func GetKubeClientFromRESTConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	return kubernetes.NewForConfig(config)
}

//runKubectlCommand ...
func (f *Framework) runKubectlCommand(args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := f.createKubectlCommand(args...)

	cmd.Stderr = &errb
	stdOutBytes, err := cmd.Output()
	if err != nil {
		if len(errb.String()) > 0 {
			return errb.String(), err
		}
	}
	return string(stdOutBytes), nil
}

// createKubectlCommand returns the Cmd to execute kubectl
func (f *Framework) createKubectlCommand(args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}
