package shared

import (
	"testing"
	"time"
)

func TestNewHealthStatus_UpWhenAllChecksUp(t *testing.T) {
	checks := map[string]string{"firestore": HealthUp, "pubsub": HealthUp}

	got := NewHealthStatus(checks)

	if got.Status != HealthUp {
		t.Fatalf("Status = %q, want %q", got.Status, HealthUp)
	}
	if len(got.Checks) != 2 {
		t.Fatalf("Checks has %d entries, want 2", len(got.Checks))
	}
}

func TestNewHealthStatus_DownWhenAnyCheckDown(t *testing.T) {
	checks := map[string]string{"firestore": HealthUp, "pubsub": HealthDown}

	got := NewHealthStatus(checks)

	if got.Status != HealthDown {
		t.Fatalf("Status = %q, want %q", got.Status, HealthDown)
	}
}

func TestNewHealthStatus_UpWithNoChecks(t *testing.T) {
	got := NewHealthStatus(nil)

	if got.Status != HealthUp {
		t.Fatalf("Status = %q, want %q", got.Status, HealthUp)
	}
}

func TestNewHealthStatus_TimestampIsRFC3339UTC(t *testing.T) {
	got := NewHealthStatus(map[string]string{})

	ts, err := time.Parse(time.RFC3339, got.Timestamp)
	if err != nil {
		t.Fatalf("Timestamp %q is not RFC3339: %v", got.Timestamp, err)
	}
	if ts.Location() != time.UTC {
		t.Fatalf("Timestamp %q is not UTC", got.Timestamp)
	}
}
