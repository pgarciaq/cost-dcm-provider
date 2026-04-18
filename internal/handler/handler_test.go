package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	oapigen "github.com/dcm-project/koku-cost-provider/internal/api/server"
	"github.com/dcm-project/koku-cost-provider/internal/health"
	"github.com/dcm-project/koku-cost-provider/internal/koku"
	"github.com/dcm-project/koku-cost-provider/internal/monitoring"
	"github.com/dcm-project/koku-cost-provider/internal/store"
	"github.com/dcm-project/koku-cost-provider/internal/util"

	"log/slog"
	"os"
	"time"
)

func setupHandler(t *testing.T) (*Handler, *httptest.Server) {
	t.Helper()

	// Mock Koku server
	kokuServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/cost-management/v1/sources/":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "src-uuid-test", "name": "test"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/cost-management/v1/cost-models/":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "cm-uuid-test"})
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": []}`))
		}
	}))

	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	kokuClient := koku.NewClient(kokuServer.URL, "test-id")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := health.NewChecker("test", time.Now(), kokuServer.URL)

	h := New(s, kokuClient, monitoring.NoopPublisher{}, checker, logger)
	return h, kokuServer
}

func TestCreateAndGetInstance(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()

	id := "test-instance-1"
	body := oapigen.CostInstance{
		Spec: oapigen.CostSpec{
			ServiceType: "cost",
			Metadata: oapigen.CostMetadata{
				Name: "my-cost-instance",
			},
			Target: oapigen.CostTarget{
				ResourceId: "cluster-abc",
			},
		},
	}

	createResp, err := h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
		Params: oapigen.CreateInstanceParams{Id: &id},
		Body:   &body,
	})
	if err != nil {
		t.Fatalf("CreateInstance error: %v", err)
	}

	created, ok := createResp.(oapigen.CreateInstance201JSONResponse)
	if !ok {
		t.Fatalf("expected 201, got %T", createResp)
	}
	if *created.Id != "test-instance-1" {
		t.Errorf("expected id test-instance-1, got %s", *created.Id)
	}
	if *created.Status != "PROVISIONING" {
		t.Errorf("expected PROVISIONING, got %s", string(*created.Status))
	}

	// Get it back
	getResp, err := h.GetInstance(ctx, oapigen.GetInstanceRequestObject{InstanceId: "test-instance-1"})
	if err != nil {
		t.Fatalf("GetInstance error: %v", err)
	}
	got, ok := getResp.(oapigen.GetInstance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", getResp)
	}
	if *got.ClusterId != "cluster-abc" {
		t.Errorf("expected cluster_id cluster-abc, got %s", *got.ClusterId)
	}
}

func TestCreateDuplicateTarget(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()
	body := oapigen.CostInstance{
		Spec: oapigen.CostSpec{
			ServiceType: "cost",
			Metadata:    oapigen.CostMetadata{Name: "a"},
			Target:      oapigen.CostTarget{ResourceId: "same-cluster"},
		},
	}

	id1 := "inst-1"
	_, _ = h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
		Params: oapigen.CreateInstanceParams{Id: &id1},
		Body:   &body,
	})

	id2 := "inst-2"
	resp, _ := h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
		Params: oapigen.CreateInstanceParams{Id: &id2},
		Body:   &body,
	})
	if _, ok := resp.(oapigen.CreateInstance409ApplicationProblemPlusJSONResponse); !ok {
		t.Fatalf("expected 409, got %T", resp)
	}
}

func TestDeleteInstance(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()
	id := "del-test"
	body := oapigen.CostInstance{
		Spec: oapigen.CostSpec{
			ServiceType: "cost",
			Metadata:    oapigen.CostMetadata{Name: "x"},
			Target:      oapigen.CostTarget{ResourceId: "cluster-del"},
		},
	}
	_, _ = h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
		Params: oapigen.CreateInstanceParams{Id: &id},
		Body:   &body,
	})

	delResp, err := h.DeleteInstance(ctx, oapigen.DeleteInstanceRequestObject{InstanceId: "del-test"})
	if err != nil {
		t.Fatalf("DeleteInstance error: %v", err)
	}
	if _, ok := delResp.(oapigen.DeleteInstance204Response); !ok {
		t.Fatalf("expected 204, got %T", delResp)
	}

	// Should show DELETED status
	getResp, _ := h.GetInstance(ctx, oapigen.GetInstanceRequestObject{InstanceId: "del-test"})
	got, ok := getResp.(oapigen.GetInstance200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", getResp)
	}
	if *got.Status != "DELETED" {
		t.Errorf("expected DELETED status, got %s", string(*got.Status))
	}
}

func TestGetNotFound(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	resp, _ := h.GetInstance(context.Background(), oapigen.GetInstanceRequestObject{InstanceId: "nope"})
	if _, ok := resp.(oapigen.GetInstance404ApplicationProblemPlusJSONResponse); !ok {
		t.Fatalf("expected 404, got %T", resp)
	}
}

func TestListInstances(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()
	for _, cid := range []string{"c1", "c2", "c3"} {
		id := "list-" + cid
		body := oapigen.CostInstance{
			Spec: oapigen.CostSpec{
				ServiceType: "cost",
				Metadata:    oapigen.CostMetadata{Name: "n"},
				Target:      oapigen.CostTarget{ResourceId: cid},
			},
		}
		_, _ = h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
			Params: oapigen.CreateInstanceParams{Id: &id},
			Body:   &body,
		})
	}

	resp, _ := h.ListInstances(ctx, oapigen.ListInstancesRequestObject{
		Params: oapigen.ListInstancesParams{MaxPageSize: util.Ptr(int32(10))},
	})
	list, ok := resp.(oapigen.ListInstances200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if list.Instances == nil || len(*list.Instances) != 3 {
		t.Errorf("expected 3 instances, got %d", len(*list.Instances))
	}
}

func TestDoubleDelete(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()
	id := "double-del"
	body := oapigen.CostInstance{
		Spec: oapigen.CostSpec{
			ServiceType: "cost",
			Metadata:    oapigen.CostMetadata{Name: "x"},
			Target:      oapigen.CostTarget{ResourceId: "cluster-dd"},
		},
	}
	_, _ = h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
		Params: oapigen.CreateInstanceParams{Id: &id},
		Body:   &body,
	})

	// First delete
	resp1, _ := h.DeleteInstance(ctx, oapigen.DeleteInstanceRequestObject{InstanceId: id})
	if _, ok := resp1.(oapigen.DeleteInstance204Response); !ok {
		t.Fatalf("first delete: expected 204, got %T", resp1)
	}

	// Second delete should also return 204 (idempotent)
	resp2, _ := h.DeleteInstance(ctx, oapigen.DeleteInstanceRequestObject{InstanceId: id})
	if _, ok := resp2.(oapigen.DeleteInstance204Response); !ok {
		t.Fatalf("second delete: expected 204, got %T", resp2)
	}
}

func TestListPagination(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()
	for _, cid := range []string{"p1", "p2", "p3"} {
		id := "page-" + cid
		body := oapigen.CostInstance{
			Spec: oapigen.CostSpec{
				ServiceType: "cost",
				Metadata:    oapigen.CostMetadata{Name: "n"},
				Target:      oapigen.CostTarget{ResourceId: cid},
			},
		}
		_, _ = h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
			Params: oapigen.CreateInstanceParams{Id: &id},
			Body:   &body,
		})
	}

	// Page 1: 2 items
	pageSize := int32(2)
	resp, _ := h.ListInstances(ctx, oapigen.ListInstancesRequestObject{
		Params: oapigen.ListInstancesParams{MaxPageSize: &pageSize},
	})
	list, ok := resp.(oapigen.ListInstances200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if list.Instances == nil || len(*list.Instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(*list.Instances))
	}
	if list.NextPageToken == nil {
		t.Fatal("expected next_page_token, got nil")
	}

	// Page 2: use the token
	resp2, _ := h.ListInstances(ctx, oapigen.ListInstancesRequestObject{
		Params: oapigen.ListInstancesParams{MaxPageSize: &pageSize, PageToken: list.NextPageToken},
	})
	list2, ok := resp2.(oapigen.ListInstances200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp2)
	}
	if list2.Instances == nil || len(*list2.Instances) != 1 {
		t.Fatalf("expected 1 instance on page 2, got %d", len(*list2.Instances))
	}
	if list2.NextPageToken != nil {
		t.Errorf("expected no next page token, got %s", *list2.NextPageToken)
	}
}

func TestCreateInvalidResourceId(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	ctx := context.Background()
	id := "bad-id"
	body := oapigen.CostInstance{
		Spec: oapigen.CostSpec{
			ServiceType: "cost",
			Metadata:    oapigen.CostMetadata{Name: "x"},
			Target:      oapigen.CostTarget{ResourceId: "../../admin/hack"},
		},
	}

	resp, _ := h.CreateInstance(ctx, oapigen.CreateInstanceRequestObject{
		Params: oapigen.CreateInstanceParams{Id: &id},
		Body:   &body,
	})
	if _, ok := resp.(oapigen.CreateInstance400ApplicationProblemPlusJSONResponse); !ok {
		t.Fatalf("expected 400, got %T", resp)
	}
}

func TestGetHealth(t *testing.T) {
	h, ts := setupHandler(t)
	defer ts.Close()

	resp, err := h.GetHealth(context.Background(), oapigen.GetHealthRequestObject{})
	if err != nil {
		t.Fatalf("GetHealth error: %v", err)
	}
	health, ok := resp.(oapigen.GetHealth200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if health.Version == nil || *health.Version != "test" {
		t.Errorf("expected version 'test'")
	}
}
