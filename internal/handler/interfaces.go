package handler

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/dcm-project/koku-cost-provider/internal/koku"
	"github.com/dcm-project/koku-cost-provider/internal/store"
)

// InstanceStore abstracts the persistent store operations.
type InstanceStore interface {
	Create(inst *store.CostInstance) error
	Update(inst *store.CostInstance) error
	Get(id string) (*store.CostInstance, error)
	GetByTarget(targetResourceID string) (*store.CostInstance, error)
	List(limit, offset int) ([]store.CostInstance, int64, error)
	UpdateStatus(id, status, message string) error
	ListByStatus(status string) ([]store.CostInstance, error)
}

// KokuClient abstracts the Koku REST API interactions.
type KokuClient interface {
	CreateSource(ctx context.Context, clusterID, name string) (*koku.SourceResponse, error)
	PauseSource(ctx context.Context, uuid string) error
	CreateCostModel(ctx context.Context, name, sourceUUID string, rates []koku.CostModelRate, markup *koku.CostModelMarkup, distribution string) (*koku.CostModelResponse, error)
	DeleteCostModel(ctx context.Context, uuid string) error
	GetSourceStats(ctx context.Context, uuid string) (koku.SourceStatsResponse, error)
	GetReports(ctx context.Context, clusterID, reportType string, params url.Values) (json.RawMessage, error)
	GetForecasts(ctx context.Context, clusterID string, params url.Values) (json.RawMessage, error)
}
