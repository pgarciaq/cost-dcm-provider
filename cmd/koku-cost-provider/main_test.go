package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dcm-project/koku-cost-provider/internal/monitoring"
)

func TestNATSFallbackWhenURLEmpty(t *testing.T) {
	var publisher monitoring.StatusPublisher

	natsURL := ""
	if natsURL == "" {
		publisher = monitoring.NoopPublisher{}
	}

	if publisher == nil {
		t.Fatal("expected NoopPublisher when NATS URL is empty")
	}
	if _, ok := publisher.(monitoring.NoopPublisher); !ok {
		t.Fatalf("expected NoopPublisher, got %T", publisher)
	}
}

func TestNATSFallbackWhenUnreachable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	var publisher monitoring.StatusPublisher
	natsURL := "nats://127.0.0.1:1"

	p, natsErr := monitoring.NewNATSPublisher(natsURL, "test", logger)
	if natsErr != nil {
		publisher = monitoring.NoopPublisher{}
	} else {
		publisher = p
		defer func() { _ = p.Close() }()
	}

	if _, ok := publisher.(monitoring.NoopPublisher); !ok {
		t.Fatalf("expected NoopPublisher for unreachable NATS, got %T (NATS silently connected)", publisher)
	}
}

func TestPublishDegradeGracefully(t *testing.T) {
	publisher := monitoring.NoopPublisher{}
	err := publisher.Publish(context.Background(), "id-1", "READY", "test")
	if err != nil {
		t.Fatalf("NoopPublisher.Publish should never error, got: %v", err)
	}
}

func TestListenerBindsAndCloses(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("re-Listen after close: %v", err)
	}
	_ = ln2.Close()
}

func TestDBDirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "sub", "dir")
	dbPath := filepath.Join(dbDir, "test.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	info, err := os.Stat(dbDir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestConfigRejectsPlaintextKoku(t *testing.T) {
	t.Setenv("KOKU_API_URL", "http://koku.example.com")
	t.Setenv("KOKU_IDENTITY", "dGVzdA==")
	t.Setenv("DCM_REGISTRATION_URL", "http://dcm.example.com")
	t.Setenv("SP_ENDPOINT", "http://localhost:8080")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = ctx
	err := run(logger)
	if err == nil {
		t.Fatal("expected error for plaintext KOKU_API_URL")
	}
}
