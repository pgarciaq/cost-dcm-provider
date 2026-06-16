package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckHealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewChecker("1.0.0", time.Now(), ts.URL)
	result := c.Check()
	if *result.Status != "healthy" {
		t.Errorf("expected healthy, got %s", *result.Status)
	}
	if *result.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", *result.Version)
	}
}

func TestCheckUnhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := NewChecker("1.0.0", time.Now(), ts.URL)
	result := c.Check()
	if *result.Status != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", *result.Status)
	}
}

func TestCheckUnreachable(t *testing.T) {
	c := NewChecker("1.0.0", time.Now(), "http://127.0.0.1:1")
	result := c.Check()
	if *result.Status != "unhealthy" {
		t.Errorf("expected unhealthy for unreachable, got %s", *result.Status)
	}
}

func TestCheckUptime(t *testing.T) {
	c := NewChecker("1.0.0", time.Now().Add(-10*time.Second), "http://127.0.0.1:1")
	result := c.Check()
	if *result.Uptime < 9 {
		t.Errorf("expected uptime >= 9s, got %d", *result.Uptime)
	}
}

func TestCheckCaching(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewChecker("1.0.0", time.Now(), ts.URL)
	c.cacheTTL = 1 * time.Hour

	_ = c.Check()
	_ = c.Check()
	_ = c.Check()

	if calls != 1 {
		t.Errorf("expected 1 probe call (cached), got %d", calls)
	}
}
