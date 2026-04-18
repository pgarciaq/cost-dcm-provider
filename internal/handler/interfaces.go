package handler

import (
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
	List(limit, offset int) ([]store.CostInstance, int64, error)
	UpdateStatus(id, status, message string) error
	Delete(id string) error
	ListByStatus(status string) ([]store.CostInstance, error)
}

// KokuClient abstracts the Koku REST API interactions.
type KokuClient interface {
	CreateSource(clusterID, name string) (*koku.SourceResponse, error)
	PauseSource(uuid string) error
	CreateCostModel(name, sourceUUID string, rates []koku.CostModelRate, markup *koku.CostModelMarkup, distribution string) (*koku.CostModelResponse, error)
	DeleteCostModel(uuid string) error
	GetSourceStats(uuid string) (koku.SourceStatsResponse, error)
	GetReports(clusterID, reportType string, params url.Values) (json.RawMessage, error)
	GetForecasts(clusterID string, params url.Values) (json.RawMessage, error)
}
