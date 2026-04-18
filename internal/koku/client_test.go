package koku

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSource(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/cost-management/v1/sources/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-rh-identity") != "test-identity" {
			t.Errorf("missing or wrong x-rh-identity header")
		}

		var req SourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Authentication.Credentials.ClusterID != "cluster-abc" {
			t.Errorf("expected cluster_id cluster-abc, got %s", req.Authentication.Credentials.ClusterID)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SourceResponse{UUID: "src-uuid-123", Name: req.Name})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-identity")
	src, err := c.CreateSource("cluster-abc", "my-source")
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	if src.UUID != "src-uuid-123" {
		t.Errorf("expected UUID src-uuid-123, got %s", src.UUID)
	}
}

func TestPauseSource(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-identity")
	if err := c.PauseSource("src-uuid"); err != nil {
		t.Fatalf("PauseSource: %v", err)
	}
}

func TestCreateCostModel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CostModelResponse{UUID: "cm-uuid-456"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-identity")
	rates := []CostModelRate{
		{
			Metric:      CostModelMetric{Name: "cpu_core_usage_per_hour"},
			CostType:    "Infrastructure",
			TieredRates: []CostModelTieredRate{{Value: 0.05, Unit: "USD"}},
		},
	}
	cm, err := c.CreateCostModel("test-model", "src-uuid", rates, nil, "cpu")
	if err != nil {
		t.Fatalf("CreateCostModel: %v", err)
	}
	if cm.UUID != "cm-uuid-456" {
		t.Errorf("expected UUID cm-uuid-456, got %s", cm.UUID)
	}
}

func TestDeleteCostModel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-identity")
	if err := c.DeleteCostModel("cm-uuid"); err != nil {
		t.Fatalf("DeleteCostModel: %v", err)
	}
}

func TestGetSourceStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{"some-stat"},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-identity")
	stats, err := c.GetSourceStats("src-uuid")
	if err != nil {
		t.Fatalf("GetSourceStats: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
}

func TestGetReports(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter[cluster]") != "cluster-id" {
			t.Errorf("missing cluster filter")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-identity")
	data, err := c.GetReports("cluster-id", "compute", nil)
	if err != nil {
		t.Fatalf("GetReports: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
}
