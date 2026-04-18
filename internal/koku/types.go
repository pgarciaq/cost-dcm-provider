package koku

// SourceRequest is the payload for POST /api/cost-management/v1/sources/.
type SourceRequest struct {
	Name           string          `json:"name"`
	SourceType     string          `json:"source_type"`
	Authentication *Authentication `json:"authentication"`
	BillingSource  *BillingSource  `json:"billing_source"`
}

type Authentication struct {
	Credentials *Credentials `json:"credentials"`
}

type Credentials struct {
	ClusterID string `json:"cluster_id"`
}

type BillingSource struct {
	Bucket string `json:"bucket"`
}

// SourceResponse represents a Koku source.
type SourceResponse struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	Paused     bool   `json:"paused"`
}

// CostModelRequest is the payload for POST /api/cost-management/v1/cost-models/.
type CostModelRequest struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	SourceType   string           `json:"source_type"`
	SourceUUIDs  []string         `json:"source_uuids"`
	Rates        []CostModelRate  `json:"rates"`
	Markup       *CostModelMarkup `json:"markup,omitempty"`
	Distribution string           `json:"distribution,omitempty"`
}

type CostModelRate struct {
	Metric      CostModelMetric       `json:"metric"`
	CostType    string                `json:"cost_type"`
	TieredRates []CostModelTieredRate `json:"tiered_rates"`
}

type CostModelMetric struct {
	Name string `json:"name"`
}

type CostModelTieredRate struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type CostModelMarkup struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// CostModelResponse represents a Koku cost model.
type CostModelResponse struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// SourceStatsResponse holds stats for a Koku source.
type SourceStatsResponse map[string]any
