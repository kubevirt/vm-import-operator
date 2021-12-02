package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler creates a new prometheus handler to receive scrap requests
func Handler(MaxRequestsInFlight int) http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				MaxRequestsInFlight: MaxRequestsInFlight,
			}),
	)
}

// NewPrometheusScraper returns a new struct of the prometheus scrapper
func NewPrometheusScraper(ch chan<- prometheus.Metric) *PrometheusScraper {
	return &PrometheusScraper{ch: ch}
}

// PrometheusScraper struct containing the resources to scrap prometheus metrics
type PrometheusScraper struct {
	ch chan<- prometheus.Metric
}

// Report adds CDI metrics to PrometheusScraper
func (ps *PrometheusScraper) Report(socketFile string) {
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("collector goroutine panicked for VM %s: %s", socketFile, err)
		}
	}()

	descCh := make(chan *prometheus.Desc)
	ps.describeVec(descCh, importCounterVec)
	ps.describeVec(descCh, importDurationVec)
}

func (ps *PrometheusScraper) describeVec(descCh chan *prometheus.Desc, vec prometheus.Collector) {
	go vec.Describe(descCh)
	desc := <-descCh
	ps.newMetric(desc)
}

func (ps *PrometheusScraper) newMetric(desc *prometheus.Desc) {
	mv, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, 1024, "")
	if err != nil {
		panic(err)
	}
	ps.ch <- mv
}
