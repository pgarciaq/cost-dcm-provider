// Package registration handles self-registration of the cost service provider
// with the DCM Service Provider Manager.
package registration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	spmv1alpha1 "github.com/dcm-project/service-provider-manager/api/v1alpha1/provider"
	spmclient "github.com/dcm-project/service-provider-manager/pkg/client/provider"

	"github.com/dcm-project/koku-cost-provider/internal/config"
)

type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}

type Registrar struct {
	cfg       config.RegistrationConfig
	dcmClient *spmclient.ClientWithResponses
	logger    *slog.Logger
	startOnce sync.Once
	done      chan struct{}
}

func New(cfg config.RegistrationConfig, dcmClient *spmclient.ClientWithResponses, logger *slog.Logger) *Registrar {
	return &Registrar{
		cfg:       cfg,
		dcmClient: dcmClient,
		logger:    logger,
		done:      make(chan struct{}),
	}
}

func (r *Registrar) Start(ctx context.Context) {
	r.startOnce.Do(func() {
		go func() {
			defer close(r.done)
			r.registerWithRetry(ctx)
		}()
	})
}

func (r *Registrar) Done() <-chan struct{} { return r.done }

func (r *Registrar) Register(ctx context.Context) error {
	ops := []string{"CREATE", "DELETE", "READ"}
	metadata := &spmv1alpha1.ProviderMetadata{}
	metadata.Set("backingService", "koku")
	metadata.Set("supportedProviderTypes", []string{"OCP"})

	provider := spmv1alpha1.Provider{
		Name:          r.cfg.ProviderName,
		ServiceType:   "cost",
		SchemaVersion: "v1alpha1",
		Endpoint:      r.cfg.Endpoint,
		Operations:    &ops,
		Metadata:      metadata,
	}
	if r.cfg.DisplayName != "" {
		provider.DisplayName = &r.cfg.DisplayName
	}

	resp, err := r.dcmClient.CreateProviderWithResponse(ctx, nil, provider)
	if err != nil {
		return &retryableError{err: fmt.Errorf("calling DCM registry: %w", err)}
	}

	code := resp.StatusCode()
	switch {
	case code == http.StatusOK || code == http.StatusCreated:
		return nil
	case code >= 400 && code < 500:
		return fmt.Errorf("registration rejected: status %d: %s", code, truncateBody(resp.Body))
	default:
		return &retryableError{err: fmt.Errorf("registration failed: status %d: %s", code, truncateBody(resp.Body))}
	}
}

func (r *Registrar) registerWithRetry(ctx context.Context) {
	backoff := r.cfg.InitialBackoff
	for {
		err := r.Register(ctx)
		if err == nil {
			r.logger.Info("registration successful")
			return
		}
		if !isRetryable(err) {
			r.logger.Error("registration failed with non-retryable error", "error", err)
			return
		}
		r.logger.Warn("registration failed, will retry", "error", err, "retry_in", backoff)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff *= 2
		if backoff > r.cfg.MaxBackoff {
			backoff = r.cfg.MaxBackoff
		}
	}
}

func truncateBody(body []byte) string {
	const maxLen = 200
	if len(body) > maxLen {
		return string(body[:maxLen])
	}
	return string(body)
}
