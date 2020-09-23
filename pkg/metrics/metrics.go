package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

// Return current value of counter. If error is not nil then value is undefined
func (ic *importCounter) getValue(labels prometheus.Labels) (float64, error) {
	var m = &dto.Metric{}
	err := ic.importCounterVec.With(labels).Write(m)
	return m.Counter.GetValue(), err
}

// IncFailed increment failed label
func (ic *importCounter) IncFailed() {
	ic.importCounterVec.With(prometheus.Labels{"result": "failed"}).Inc()
}

func (ic *importCounter) GetFailed() (float64, error) {
	return ic.getValue(prometheus.Labels{"result": "failed"})
}

// IncSuccessful increment successfull label
func (ic *importCounter) IncSuccessful() {
	ic.importCounterVec.With(prometheus.Labels{"result": "successful"}).Inc()
}

func (ic *importCounter) GetSuccessful() (float64, error) {
	return ic.getValue(prometheus.Labels{"result": "successful"})
}

// IncCancelled increment successfull label
func (ic *importCounter) IncCancelled() {
	ic.importCounterVec.With(prometheus.Labels{"result": "cancelled"}).Inc()
}

func (ic *importCounter) GetCancelled() (float64, error) {
	return ic.getValue(prometheus.Labels{"result": "cancelled"})
}
