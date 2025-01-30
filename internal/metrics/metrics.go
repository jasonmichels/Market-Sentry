package metrics

import "github.com/prometheus/client_golang/prometheus"

// Example metrics
var (
	TotalRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "myapp_total_requests",
			Help: "Total number of HTTP requests received",
		},
	)

	// You could add more metrics: histograms for request latency, gauge for # of alerts, etc.
)

func init() {
	prometheus.MustRegister(TotalRequests)
}
