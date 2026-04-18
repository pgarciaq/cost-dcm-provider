package reconciler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/dcm-project/koku-cost-provider/internal/koku"
	"github.com/dcm-project/koku-cost-provider/internal/store"

	"log/slog"
	"os"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(filepath.Join(t.TempDir(), "recon_test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return s
}

func TestReconcileTransitionsToReady(t *testing.T) {
	kokuSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return non-empty stats to indicate data received
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{"something"},
		})
	}))
	defer kokuSrv.Close()

	s := newTestStore(t)
	kokuClient := koku.NewClient(kokuSrv.URL, "test-id")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	pub := &recordingPublisher{}

	_ = s.Create(&store.CostInstance{
		ID:               "recon-1",
		TargetResourceID: "cluster-r1",
		ClusterID:        "cluster-r1",
		KokuSourceUUID:   "src-uuid-1",
		Status:           "PROVISIONING",
		StatusMessage:    "waiting",
	})

	r := New(s, kokuClient, pub, logger, 100*time.Millisecond, 1*time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go r.Start(ctx)

	// Wait for reconciler to transition
	deadline := time.After(3 * time.Second)
	for {
		inst, err := s.Get("recon-1")
		if err == nil && inst.Status == "READY" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for instance to transition to READY")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	cancel()
	<-r.Done()

	if len(pub.events) == 0 {
		t.Error("expected at least one published event")
	}
	if pub.events[len(pub.events)-1].status != "READY" {
		t.Errorf("expected last event status=READY, got %s", pub.events[len(pub.events)-1].status)
	}
}

func TestReconcileTimesOut(t *testing.T) {
	kokuSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer kokuSrv.Close()

	s := newTestStore(t)
	kokuClient := koku.NewClient(kokuSrv.URL, "test-id")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	pub := &recordingPublisher{}

	// Created far in the past to trigger timeout
	inst := &store.CostInstance{
		ID:               "recon-timeout",
		TargetResourceID: "cluster-rt",
		ClusterID:        "cluster-rt",
		KokuSourceUUID:   "src-uuid-t",
		Status:           "PROVISIONING",
		StatusMessage:    "waiting",
	}
	_ = s.Create(inst)
	// Manually backdate the created_at to trigger timeout
	_ = s.UpdateStatus(inst.ID, "PROVISIONING", "waiting")

	r := New(s, kokuClient, pub, logger, 100*time.Millisecond, 1*time.Nanosecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go r.Start(ctx)

	deadline := time.After(3 * time.Second)
	for {
		got, err := s.Get("recon-timeout")
		if err == nil && got.Status == "ERROR" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for instance to transition to ERROR")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	cancel()
	<-r.Done()
}

func TestReconcileSkipsNoKokuSource(t *testing.T) {
	s := newTestStore(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	pub := &recordingPublisher{}

	_ = s.Create(&store.CostInstance{
		ID:               "no-source",
		TargetResourceID: "cluster-ns",
		ClusterID:        "cluster-ns",
		KokuSourceUUID:   "",
		Status:           "PROVISIONING",
		StatusMessage:    "waiting",
	})

	r := New(s, koku.NewClient("http://unused", "x"), pub, logger, 50*time.Millisecond, 1*time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go r.Start(ctx)
	<-r.Done()

	inst, _ := s.Get("no-source")
	if inst.Status != "PROVISIONING" {
		t.Errorf("expected PROVISIONING, got %s", inst.Status)
	}
	if len(pub.events) != 0 {
		t.Errorf("expected no events, got %d", len(pub.events))
	}
}

func TestHasData(t *testing.T) {
	tests := []struct {
		name   string
		input  koku.SourceStatsResponse
		expect bool
	}{
		{"nil", nil, false},
		{"empty map", koku.SourceStatsResponse{}, false},
		{"non-empty array", koku.SourceStatsResponse{"k": []any{"v"}}, true},
		{"empty array", koku.SourceStatsResponse{"k": []any{}}, false},
		{"non-empty nested map", koku.SourceStatsResponse{"k": map[string]any{"a": 1}}, true},
		{"empty nested map", koku.SourceStatsResponse{"k": map[string]any{}}, false},
		{"string value", koku.SourceStatsResponse{"k": "str"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasData(tt.input); got != tt.expect {
				t.Errorf("hasData(%v) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

type publishedEvent struct {
	instanceID string
	status     string
	message    string
}

type recordingPublisher struct {
	events []publishedEvent
}

func (p *recordingPublisher) Publish(_ context.Context, instanceID, status, message string) error {
	p.events = append(p.events, publishedEvent{instanceID, status, message})
	return nil
}

func (p *recordingPublisher) Close() error { return nil }
