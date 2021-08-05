package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	ctrlConfig "github.com/kubevirt/vm-import-operator/pkg/config/controller"

	"k8s.io/client-go/kubernetes"

	clientutil "kubevirt.io/client-go/util"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"github.com/kubevirt/vm-import-operator/pkg/apis"
	"github.com/kubevirt/vm-import-operator/pkg/controller"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/kubevirt/vm-import-operator/pkg/metrics"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	kubemetrics "github.com/operator-framework/operator-sdk/pkg/kube-metrics"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

// Change below variables to serve metrics on different host or port.
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()
	// Become the leader before proceeding
	err = leader.Become(ctx, "vm-import-operator-lock")
	if err != nil {
		log.Error(err, "")
		if os.Getenv("DEV_LOCAL_DEBUG") == "" {
			os.Exit(1)
		}
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metrics.MetricsHost, metrics.MetricsPort),
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup Scheme for kubevirt resources
	if err := kubevirtv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup Scheme for kubevirt resources
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup Scheme for netv1 resources
	if err := netv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	kubevirtNamespace, err := clientutil.GetNamespace()
	if err != nil {
		log.Error(err, "Cannot get operator's namespace")
		os.Exit(1)
	}

	k8sClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	stop := make(chan struct{})
	defer close(stop)

	ctrlConfigProvider := ctrlConfig.NewControllerConfigProvider(stop, k8sClient, kubevirtNamespace)

	// Setup all Controllers
	if err := controller.AddToManager(mgr, &ctrlConfigProvider); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err = serveCRMetrics(cfg); err != nil {
		log.Info("Could not generate and serve custom resource metrics", "error", err.Error())
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// serveCRMetrics gets the Operator/CustomResource GVKs and generates metrics based on those types.
// It serves those metrics on "http://metricsHost:operatorMetricsPort".
func serveCRMetrics(cfg *rest.Config) error {
	// Below function returns filtered operator/CustomResource specific GVKs.
	// For more control override the below GVK list with your own custom logic.
	filteredGVK, err := k8sutil.GetGVKsFromAddToScheme(apis.AddToScheme)
	if err != nil {
		return err
	}

	// group v2v.kubevirt.io might be too extensive for controller needs, we need to minimize
	// the monitored items to the specific types the controller concerns
	controllerGVKs := filteredGVK[:0]
	for _, gvk := range filteredGVK {
		if gvk.Kind == "ResourceMapping" || gvk.Kind == "VirtualMachineImport" {
			controllerGVKs = append(controllerGVKs, gvk)
		}
	}

	// Get the namespace the operator is currently deployed in.
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}
	// To generate metrics in other namespaces, add the values below.
	ns := []string{operatorNs}
	// Generate and serve custom resource specific metrics.
	err = kubemetrics.GenerateAndServeCRMetrics(cfg, ns, controllerGVKs, metrics.MetricsHost, metrics.OperatorMetricsPort)
	if err != nil {
		return err
	}
	return nil
}
