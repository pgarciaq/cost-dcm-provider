package koku

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func noopBackoff(_ context.Context, _ time.Duration) {}

func newTestClient(url string) *Client {
	c := NewClient(url, "test")
	c.backoffFn = noopBackoff
	return c
}

func TestRetryOn500(t *testing.T) {
	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	resp, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRetryExhausted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	resp, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
}

func TestRetryRespectsContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := newTestClient(ts.URL)
	resp, err := c.doWithRetry(ctx, http.MethodGet, "/test", nil)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	resp, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("4xx should not cause retry error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if attempts.Load() != 1 {
		t.Errorf("expected exactly 1 attempt for 4xx, got %d", attempts.Load())
	}
}
