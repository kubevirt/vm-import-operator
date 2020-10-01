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
	// Histogram for duration of imports
	importDurationVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kubevirt_vmimport_duration",
			Help:    "Satistics of virtual machine import duration",
			Buckets: []float64{60, 15 * 60, 60 * 60}, // 1 minute, 15 minutesm 1 hour and above 1 hour duration
		},
		[]string{"result"},
	)
	// ImportMetrics wrapper for all import metrics
	ImportMetrics = importMetrics{importCounterVec, importDurationVec}
)

func init() {
	metrics.Registry.MustRegister(importCounterVec, importDurationVec)
}

// importMetrics holds all metrics
type importMetrics struct {
	importCounterVec  *prometheus.CounterVec
	importDurationVec *prometheus.HistogramVec
}

// Return current value of counter. If error is not nil then value is undefined
func (ic *importMetrics) getValue(labels prometheus.Labels) (float64, error) {
	var m = &dto.Metric{}
	err := ic.importCounterVec.With(labels).Write(m)
	return m.Counter.GetValue(), err
}

// IncFailed increment failed label
func (ic *importMetrics) IncFailed() {
	ic.importCounterVec.With(prometheus.Labels{"result": "failed"}).Inc()
}

func (ic *importMetrics) GetFailed() (float64, error) {
	return ic.getValue(prometheus.Labels{"result": "failed"})
}

// IncSuccessful increment successfull label
func (ic *importMetrics) IncSuccessful() {
	ic.importCounterVec.With(prometheus.Labels{"result": "successful"}).Inc()
}

func (ic *importMetrics) GetSuccessful() (float64, error) {
	return ic.getValue(prometheus.Labels{"result": "successful"})
}

// IncCancelled increment successfull label
func (ic *importMetrics) IncCancelled() {
	ic.importCounterVec.With(prometheus.Labels{"result": "cancelled"}).Inc()
}

func (ic *importMetrics) GetCancelled() (float64, error) {
	return ic.getValue(prometheus.Labels{"result": "cancelled"})
}

// SaveDurationFailed
func (ic *importMetrics) SaveDurationFailed(d float64) {
	ic.importDurationVec.With(prometheus.Labels{"result": "failed"}).Observe(d)
}

// SaveDurationSuccessful
func (ic *importMetrics) SaveDurationSuccessful(d float64) {
	ic.importDurationVec.With(prometheus.Labels{"result": "successful"}).Observe(d)
}

// SaveDurationCancelled
func (ic *importMetrics) SaveDurationCancelled(d float64) {
	ic.importDurationVec.With(prometheus.Labels{"result": "cancelled"}).Observe(d)
}

// getCountDurationSamples returns number of duration samples for given label
func (ic *importMetrics) getCountDurationSamples(labels prometheus.Labels) (uint64, error) {
	var m = &dto.Metric{}

	err := ic.importDurationVec.With(labels).(prometheus.Histogram).Write(m)

	return m.Histogram.GetSampleCount(), err
}

// GetCountDurationSuccessful returns number of duration samples for successful imports
func (ic *importMetrics) GetCountDurationSuccessful() (uint64, error) {
	return ic.getCountDurationSamples(prometheus.Labels{"result": "successful"})
}

// GetCountDurationFailed returns number of duration samples for successful imports
func (ic *importMetrics) GetCountDurationFailed() (uint64, error) {
	return ic.getCountDurationSamples(prometheus.Labels{"result": "failed"})
}

// GetCountDurationCancelled returns number of duration samples for successful imports
func (ic *importMetrics) GetCountDurationCancelled() (uint64, error) {
	return ic.getCountDurationSamples(prometheus.Labels{"result": "cancelled"})
}
