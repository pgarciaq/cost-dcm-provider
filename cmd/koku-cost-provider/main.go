package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	oapigen "github.com/dcm-project/koku-cost-provider/internal/api/server"
	"github.com/dcm-project/koku-cost-provider/internal/apiserver"
	"github.com/dcm-project/koku-cost-provider/internal/config"
	"github.com/dcm-project/koku-cost-provider/internal/handler"
	"github.com/dcm-project/koku-cost-provider/internal/health"
	"github.com/dcm-project/koku-cost-provider/internal/koku"
	"github.com/dcm-project/koku-cost-provider/internal/monitoring"
	"github.com/dcm-project/koku-cost-provider/internal/reconciler"
	"github.com/dcm-project/koku-cost-provider/internal/registration"
	"github.com/dcm-project/koku-cost-provider/internal/store"
	spmclient "github.com/dcm-project/service-provider-manager/pkg/client/provider"
)

var version = "0.0.1-dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	logger.Info("configuration loaded",
		"bind_address", cfg.Server.BindAddress,
		"db_path", cfg.Store.DBPath,
		"koku_api_url", cfg.Koku.APIURL,
		"koku_identity", cfg.Koku.RedactedIdentity(),
		"nats_url", cfg.NATS.URL,
		"dcm_url", cfg.Registration.DCMURL,
		"provider_name", cfg.Registration.ProviderName,
		"reconciler_interval", cfg.Reconciler.PollInterval.String(),
	)

	ln, err := net.Listen("tcp", cfg.Server.BindAddress)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.Server.BindAddress, err)
	}
	defer func() { _ = ln.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Database
	if err := os.MkdirAll(filepath.Dir(cfg.Store.DBPath), 0o755); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}
	db, err := store.New(cfg.Store.DBPath)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}

	// Koku client
	kokuClient := koku.NewClient(cfg.Koku.APIURL, cfg.Koku.Identity)

	// NATS publisher
	publisher, err := monitoring.NewNATSPublisher(cfg.NATS.URL, cfg.Registration.ProviderName, logger)
	if err != nil {
		return fmt.Errorf("creating NATS publisher: %w", err)
	}
	defer func() { _ = publisher.Close() }()

	// DCM registration client
	dcmClient, err := spmclient.NewClientWithResponses(cfg.Registration.DCMURL)
	if err != nil {
		return fmt.Errorf("creating DCM client: %w", err)
	}
	registrar := registration.New(cfg.Registration, dcmClient, logger)

	// Background reconciler
	recon := reconciler.New(db, kokuClient, publisher, logger, cfg.Reconciler.PollInterval, cfg.Reconciler.ProvisionTimeout)

	// Health checker
	startTime := time.Now()
	checker := health.NewChecker(version, startTime, cfg.Koku.APIURL)

	// Handler + strict server
	h := handler.New(db, kokuClient, publisher, checker, logger)
	strictHandler := oapigen.NewStrictHandler(h, nil)
	srv := apiserver.New(cfg, logger, strictHandler).WithOnReady(func(ctx context.Context) {
		registrar.Start(ctx)
		go recon.Start(ctx)
	})

	err = srv.Run(ctx, ln)

	// Wait for the reconciler to drain before exiting.
	select {
	case <-recon.Done():
	case <-time.After(5 * time.Second):
		logger.Warn("reconciler did not shut down within 5s")
	}

	return err
}
