// Package reconciler polls provisioning instances and transitions them to
// READY once Koku reports metering data, or to ERROR on timeout.
package reconciler

import (
	"context"
	"log/slog"
	"time"

	"github.com/dcm-project/koku-cost-provider/internal/koku"
	"github.com/dcm-project/koku-cost-provider/internal/monitoring"
	"github.com/dcm-project/koku-cost-provider/internal/store"
)

// Reconciler polls Koku for PROVISIONING instances and transitions them to
// READY once first metering data is received.
type Reconciler struct {
	store     *store.Store
	koku      *koku.Client
	publisher monitoring.StatusPublisher
	logger    *slog.Logger
	interval  time.Duration
	timeout   time.Duration
	done      chan struct{}
}

func New(s *store.Store, k *koku.Client, pub monitoring.StatusPublisher, logger *slog.Logger, interval, timeout time.Duration) *Reconciler {
	return &Reconciler{
		store:     s,
		koku:      k,
		publisher: pub,
		logger:    logger,
		interval:  interval,
		timeout:   timeout,
		done:      make(chan struct{}),
	}
}

// Start runs the reconciliation loop until the context is cancelled.
// Done() is closed when the loop exits.
func (r *Reconciler) Start(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("reconciler shutting down")
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// Done returns a channel that is closed when the reconciler loop exits.
func (r *Reconciler) Done() <-chan struct{} {
	return r.done
}

func (r *Reconciler) reconcile(ctx context.Context) {
	instances, err := r.store.ListByStatus("PROVISIONING")
	if err != nil {
		r.logger.Error("failed to list provisioning instances", "error", err)
		return
	}

	for _, inst := range instances {
		if ctx.Err() != nil {
			return
		}
		r.reconcileOne(ctx, inst)
	}
}

func (r *Reconciler) reconcileOne(ctx context.Context, inst store.CostInstance) {
	if inst.KokuSourceUUID == "" {
		return
	}

	if r.timeout > 0 && time.Since(inst.CreatedAt) > r.timeout {
		r.logger.Warn("instance provision timed out", "id", inst.ID, "created_at", inst.CreatedAt)
		if err := r.store.UpdateStatus(inst.ID, "ERROR", "provision timeout: no metering data received"); err != nil {
			r.logger.Error("failed to update status to ERROR", "id", inst.ID, "error", err)
			return
		}
		if err := r.publisher.Publish(ctx, inst.ID, "ERROR", "provision timeout: no metering data received"); err != nil {
			r.logger.Warn("failed to publish ERROR event", "id", inst.ID, "error", err)
		}
		return
	}

	stats, err := r.koku.GetSourceStats(inst.KokuSourceUUID)
	if err != nil {
		r.logger.Warn("failed to get source stats", "id", inst.ID, "koku_source", inst.KokuSourceUUID, "error", err)
		return
	}

	if hasData(stats) {
		r.logger.Info("metering data received, transitioning to READY", "id", inst.ID)
		if err := r.store.UpdateStatus(inst.ID, "READY", "metering data received"); err != nil {
			r.logger.Error("failed to update status to READY", "id", inst.ID, "error", err)
			return
		}
		if err := r.publisher.Publish(ctx, inst.ID, "READY", "metering data received"); err != nil {
			r.logger.Warn("failed to publish READY event", "id", inst.ID, "error", err)
		}
	}
}

func hasData(stats koku.SourceStatsResponse) bool {
	if stats == nil {
		return false
	}
	for _, v := range stats {
		switch val := v.(type) {
		case []any:
			if len(val) > 0 {
				return true
			}
		case map[string]any:
			if len(val) > 0 {
				return true
			}
		}
	}
	return false
}
