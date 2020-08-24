package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// MetricsHost host that servers the metrics
	MetricsHost = "0.0.0.0"
	// MetricsPort port where metrics are served
	MetricsPort int32 = 8383
	// OperatorMetricsPort port where operator metrics are served
	OperatorMetricsPort int32 = 8686
)

var (
	// counter vector which count the number of vm imports done
	importCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubevirt_vmimport_counter",
			Help: "Count of virtual machine import done",
		},
		[]string{"result"},
	)
	// ImportCounter counter which hold number of vm imports done
	ImportCounter = importCounter{importCounterVec}
)

func init() {
	metrics.Registry.MustRegister(importCounterVec)
}

// ImportCounter hold the importCounter counter vector
type importCounter struct {
	importCounterVec *prometheus.CounterVec
}

// IncFailed increment failed label
func (ic *importCounter) IncFailed(providerName string) {
	ic.importCounterVec.With(prometheus.Labels{"result": "failed", "provider": providerName}).Inc()
}

// IncSuccessful increment successfull label
func (ic *importCounter) IncSuccessful(providerName string) {
	ic.importCounterVec.With(prometheus.Labels{"result": "successful", "provider": providerName}).Inc()
}
