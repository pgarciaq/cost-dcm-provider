// Package metrics defines and provides Prometheus metrics for the cost-dcm-provider.
package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dcm_cost",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests by method, route, and status code.",
	}, []string{"method", "route", "code"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "dcm_cost",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route"})

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

type metricsWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *metricsWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Middleware records request count and duration for every HTTP request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mw := &metricsWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(mw, r)
		route := NormalizePath(r.URL.Path)
		RequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(mw.statusCode)).Inc()
		RequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// NormalizePath replaces UUID and ID-like path segments with placeholders
// to prevent unbounded Prometheus label cardinality.
//
// Known routes:
//
//	/health -> /health
//	/metrics -> /metrics
//	/instances -> /instances
//	/instances/{id} -> /instances/{id}
//	/instances/{id}/usage/{metric} -> /instances/{id}/usage/{metric}
//	/instances/{id}/cost-report -> /instances/{id}/cost-report
//	/instances/{id}/forecast -> /instances/{id}/forecast
func NormalizePath(path string) string {
	path = strings.TrimRight(path, "/")
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "" {
			continue
		}
		if isStaticSegment(part) {
			continue
		}
		parts[i] = "{id}"
	}
	normalized := strings.Join(parts, "/")
	if normalized == "" {
		return "/"
	}
	return normalized
}

var staticSegments = map[string]bool{
	"health":      true,
	"metrics":     true,
	"instances":   true,
	"usage":       true,
	"cost-report": true,
	"forecast":    true,
	"api":         true,
	"v1alpha1":    true,
}

func isStaticSegment(s string) bool {
	return staticSegments[s]
}

// NormalizeKokuOp extracts a stable operation label from a Koku API path.
// e.g. "GET /api/cost-management/v1/sources/uuid-123/stats/" -> "GET sources/stats"
func NormalizeKokuOp(method, path string) string {
	path = strings.TrimRight(path, "/")
	parts := strings.Split(path, "/")

	var significant []string
	for _, p := range parts {
		if p == "" || p == "api" || p == "cost-management" || p == "v1" || p == "openshift" {
			continue
		}
		if looksLikeID(p) {
			continue
		}
		significant = append(significant, p)
	}
	if len(significant) == 0 {
		return method + " /unknown"
	}
	return method + " " + strings.Join(significant, "/")
}

func looksLikeID(s string) bool {
	if len(s) < 8 {
		return false
	}
	dashes := 0
	for _, c := range s {
		if c == '-' {
			dashes++
		}
	}
	return dashes >= 3
}
