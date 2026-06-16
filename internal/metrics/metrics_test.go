package metrics

import "testing"

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/health", "/health"},
		{"/health/", "/health"},
		{"/metrics", "/metrics"},
		{"/instances", "/instances"},
		{"/instances/", "/instances"},
		{"/instances/abc-123-def-456-789", "/instances/{id}"},
		{"/instances/abc-123-def-456-789/usage/compute", "/instances/{id}/usage/{id}"},
		{"/instances/abc-123-def-456-789/cost-report", "/instances/{id}/cost-report"},
		{"/instances/abc-123-def-456-789/forecast", "/instances/{id}/forecast"},
		{"/", "/"},
	}
	for _, tt := range tests {
		got := NormalizePath(tt.input)
		if got != tt.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeKokuOp(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"POST", "/api/cost-management/v1/sources/", "POST sources"},
		{"GET", "/api/cost-management/v1/sources/abc-def-123-456-789/stats/", "GET sources/stats"},
		{"POST", "/api/cost-management/v1/cost-models/", "POST cost-models"},
		{"DELETE", "/api/cost-management/v1/cost-models/abc-def-123-456-789/", "DELETE cost-models"},
		{"GET", "/api/cost-management/v1/reports/openshift/costs/", "GET reports/costs"},
		{"GET", "/api/cost-management/v1/forecasts/openshift/costs/", "GET forecasts/costs"},
	}
	for _, tt := range tests {
		got := NormalizeKokuOp(tt.method, tt.path)
		if got != tt.want {
			t.Errorf("NormalizeKokuOp(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
		}
	}
}
