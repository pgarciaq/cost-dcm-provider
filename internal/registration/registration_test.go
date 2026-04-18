package registration

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dcm-project/koku-cost-provider/internal/config"
	spmclient "github.com/dcm-project/service-provider-manager/pkg/client/provider"
)

func TestRegisterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "koku-cost-provider"})
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dcmClient, err := spmclient.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	reg := New(config.RegistrationConfig{
		DCMURL:         srv.URL,
		ProviderName:   "koku-cost-provider",
		Endpoint:       "http://localhost:8080",
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}, dcmClient, logger)

	err = reg.Register(context.Background())
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
}

func TestRegisterClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"bad request"}`))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dcmClient, _ := spmclient.NewClientWithResponses(srv.URL)

	reg := New(config.RegistrationConfig{
		DCMURL:         srv.URL,
		ProviderName:   "koku-cost-provider",
		Endpoint:       "http://localhost:8080",
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}, dcmClient, logger)

	err := reg.Register(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if isRetryable(err) {
		t.Error("4xx errors should not be retryable")
	}
}

func TestRegisterServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"server error"}`))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dcmClient, _ := spmclient.NewClientWithResponses(srv.URL)

	reg := New(config.RegistrationConfig{
		DCMURL:         srv.URL,
		ProviderName:   "koku-cost-provider",
		Endpoint:       "http://localhost:8080",
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}, dcmClient, logger)

	err := reg.Register(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isRetryable(err) {
		t.Error("5xx errors should be retryable")
	}
}

func TestStartWithRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "koku-cost-provider"})
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dcmClient, _ := spmclient.NewClientWithResponses(srv.URL)

	reg := New(config.RegistrationConfig{
		DCMURL:         srv.URL,
		ProviderName:   "koku-cost-provider",
		Endpoint:       "http://localhost:8080",
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}, dcmClient, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg.Start(ctx)

	select {
	case <-reg.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("registration did not complete within timeout")
	}

	if attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts)
	}
}

func TestStartCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dcmClient, _ := spmclient.NewClientWithResponses(srv.URL)

	reg := New(config.RegistrationConfig{
		DCMURL:         srv.URL,
		ProviderName:   "koku-cost-provider",
		Endpoint:       "http://localhost:8080",
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Second,
	}, dcmClient, logger)

	ctx, cancel := context.WithCancel(context.Background())
	reg.Start(ctx)

	// Cancel immediately
	cancel()

	select {
	case <-reg.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("registration goroutine did not exit after context cancellation")
	}
}
