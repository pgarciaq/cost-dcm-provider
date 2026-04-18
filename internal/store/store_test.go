package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	return s
}

func TestCreateAndGet(t *testing.T) {
	s := tempDB(t)
	inst := &CostInstance{
		ID:               "test-1",
		TargetResourceID: "cluster-a",
		ClusterID:        "ocp-cluster-id",
		KokuSourceUUID:   "src-uuid",
		Status:           "PROVISIONING",
		Name:             "my-cost",
	}
	if err := s.Create(inst); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get("test-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ClusterID != "ocp-cluster-id" {
		t.Errorf("expected cluster_id ocp-cluster-id, got %s", got.ClusterID)
	}
	if got.Status != "PROVISIONING" {
		t.Errorf("expected PROVISIONING, got %s", got.Status)
	}
}

func TestCreateDuplicate(t *testing.T) {
	s := tempDB(t)
	inst := &CostInstance{
		ID:               "test-1",
		TargetResourceID: "cluster-a",
		Status:           "PROVISIONING",
	}
	if err := s.Create(inst); err != nil {
		t.Fatalf("first create: %v", err)
	}

	inst2 := &CostInstance{
		ID:               "test-2",
		TargetResourceID: "cluster-a",
		Status:           "PROVISIONING",
	}
	err := s.Create(inst2)
	if err != ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestGetNotFound(t *testing.T) {
	s := tempDB(t)
	_, err := s.Get("nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateStatus(t *testing.T) {
	s := tempDB(t)
	_ = s.Create(&CostInstance{
		ID:               "test-1",
		TargetResourceID: "cluster-a",
		Status:           "PROVISIONING",
	})

	if err := s.UpdateStatus("test-1", "READY", "data received"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.Get("test-1")
	if got.Status != "READY" {
		t.Errorf("expected READY, got %s", got.Status)
	}
}

func TestListByStatus(t *testing.T) {
	s := tempDB(t)
	_ = s.Create(&CostInstance{ID: "a", TargetResourceID: "c1", Status: "PROVISIONING"})
	_ = s.Create(&CostInstance{ID: "b", TargetResourceID: "c2", Status: "READY"})
	_ = s.Create(&CostInstance{ID: "c", TargetResourceID: "c3", Status: "PROVISIONING"})

	provisioning, err := s.ListByStatus("PROVISIONING")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(provisioning) != 2 {
		t.Errorf("expected 2 provisioning, got %d", len(provisioning))
	}
}

func TestList(t *testing.T) {
	s := tempDB(t)
	_ = s.Create(&CostInstance{ID: "a", TargetResourceID: "c1", Status: "READY"})
	_ = s.Create(&CostInstance{ID: "b", TargetResourceID: "c2", Status: "READY"})

	instances, total, err := s.List(10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}
}

func TestDelete(t *testing.T) {
	s := tempDB(t)
	_ = s.Create(&CostInstance{ID: "a", TargetResourceID: "c1", Status: "READY"})

	if err := s.Delete("a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := s.Get("a")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
