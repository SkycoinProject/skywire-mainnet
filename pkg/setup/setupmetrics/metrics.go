package setupmetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/skycoin/skywire/pkg/routing"
)

// Metrics collects metrics for prometheus.
type Metrics interface {
	Collectors() []prometheus.Collector
	RecordRouteRequest(routing.BidirectionalRoute) func(*routing.EdgeRules, *error)
	RecordRouteListRequest(routing.BidirectionalRouteList) func(*routing.EdgeRulesList, *error)
}

// New returns the default implementation of Metrics.
func New(namespace string) Metrics {
	reqCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "active_request_count",
		Help:      "Current number of ongoing requests.",
	})
	reqDurations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "request_durations",
		Help:      "Histogram of request durations.",
	}, []string{"success"})

	return &metrics{
		activeRequests: reqCount,
		reqDurations:   reqDurations,
	}
}

type metrics struct {
	activeRequests prometheus.Gauge
	reqDurations   prometheus.ObserverVec
}

func (m *metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.activeRequests,
		m.reqDurations,
	}
}

func (m *metrics) RecordRouteRequest(_ routing.BidirectionalRoute) func(rules *routing.EdgeRules, err *error) {
	start := time.Now()
	m.activeRequests.Inc()

	return func(rules *routing.EdgeRules, err *error) {
		successStr := "true"
		if *err != nil {
			successStr = "false"
		}
		labels := prometheus.Labels{
			"success": successStr,
		}
		m.reqDurations.With(labels).Observe(float64(time.Since(start)))
		m.activeRequests.Dec()
	}
}

func (m *metrics) RecordRouteListRequest(_ routing.BidirectionalRouteList) func(rules *routing.EdgeRulesList, err *error) {
	start := time.Now()
	m.activeRequests.Inc()

	return func(rules *routing.EdgeRulesList, err *error) {
		successStr := "true"
		if *err != nil {
			successStr = "false"
		}
		labels := prometheus.Labels{
			"success": successStr,
		}
		m.reqDurations.With(labels).Observe(float64(time.Since(start)))
		m.activeRequests.Dec()
	}
}
