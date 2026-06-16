// Package metrics defines Prometheus metrics for the cost-dcm-provider.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dcm_cost",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "code"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "dcm_cost",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path"})

	InstancesManaged = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "dcm_cost",
		Name:      "instances_managed",
		Help:      "Number of cost instances currently tracked.",
	})

	KokuRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dcm_cost",
		Name:      "koku_requests_total",
		Help:      "Total upstream Koku API calls by operation and result.",
	}, []string{"operation", "result"})

	NATSEventsPublished = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "dcm_cost",
		Name:      "nats_events_published_total",
		Help:      "Total NATS CloudEvents published.",
	})
)
