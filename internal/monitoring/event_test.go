package monitoring

import (
	"testing"
	"time"
)

func TestNewStatusCloudEvent(t *testing.T) {
	before := time.Now().UTC()
	ce := NewStatusCloudEvent("my-sp", "instance-123", "READY", "data received")
	after := time.Now().UTC()

	if ce.SpecVersion != "1.0" {
		t.Errorf("expected specversion 1.0, got %s", ce.SpecVersion)
	}
	if ce.Type != "dcm.status.cost" {
		t.Errorf("expected type dcm.status.cost, got %s", ce.Type)
	}
	if ce.Source != "dcm/providers/my-sp" {
		t.Errorf("expected source dcm/providers/my-sp, got %s", ce.Source)
	}
	payload, ok := ce.Data.(StatusPayload)
	if !ok {
		t.Fatalf("expected StatusPayload, got %T", ce.Data)
	}
	if payload.ID != "instance-123" {
		t.Errorf("expected id instance-123, got %s", payload.ID)
	}
	if payload.Status != "READY" {
		t.Errorf("expected status READY, got %s", payload.Status)
	}
	if payload.Message != "data received" {
		t.Errorf("expected message 'data received', got %s", payload.Message)
	}
	if payload.Timestamp.Before(before) || payload.Timestamp.After(after) {
		t.Errorf("expected timestamp between %v and %v, got %v", before, after, payload.Timestamp)
	}
}
