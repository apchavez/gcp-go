package repos

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/testutil"
)

func TestEncodeDecodeCursor_RoundTrips(t *testing.T) {
	cursor := encodeCursor("doc-123")

	got, err := decodeCursor(cursor)

	if err != nil {
		t.Fatalf("decodeCursor() error = %v, want nil", err)
	}
	if got != "doc-123" {
		t.Fatalf("decodeCursor() = %q, want %q", got, "doc-123")
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := decodeCursor("not-valid-base64url!!!")

	if err == nil {
		t.Fatal("decodeCursor() error = nil, want an error for invalid base64")
	}
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64url, but the decoded bytes aren't a JSON object.
	_, err := decodeCursor("bm90LWpzb24")

	if err == nil {
		t.Fatal("decodeCursor() error = nil, want an error for malformed JSON payload")
	}
}

func TestApplyCursor_NoOpWhenCursorEmpty(t *testing.T) {
	r := &FirestoreStateRepo{}
	query := firestore.Query{}

	_, err := r.applyCursor(context.Background(), query, "")

	if err != nil {
		t.Fatalf("applyCursor() error = %v, want nil", err)
	}
}

func TestApplyCursor_InvalidCursorReturnsError(t *testing.T) {
	r := &FirestoreStateRepo{}
	query := firestore.Query{}

	_, err := r.applyCursor(context.Background(), query, "not-valid-base64url!!!")

	if err == nil {
		t.Fatal("applyCursor() error = nil, want an error for an undecodable cursor")
	}
}

func newTestAppointment(uuid, insuredID string) domain.Appointment {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return domain.Appointment{
		AppointmentUUID: uuid,
		InsuredID:       insuredID,
		ScheduleID:      42,
		Status:          domain.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestFirestoreStateRepo_SaveAndFindByID(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	repo := NewFirestoreStateRepo(client)
	ctx := context.Background()
	appt := newTestAppointment("save-find-001", "insured-save-find")

	if err := repo.Save(ctx, appt); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	got, err := repo.FindByID(ctx, appt.AppointmentUUID)
	if err != nil {
		t.Fatalf("FindByID() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("FindByID() = nil, want the saved appointment")
	}
	if got.AppointmentUUID != appt.AppointmentUUID || got.InsuredID != appt.InsuredID {
		t.Fatalf("FindByID() = %+v, want %+v", got, appt)
	}
}

func TestFirestoreStateRepo_FindByID_NotFoundReturnsNilNil(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	repo := NewFirestoreStateRepo(client)

	got, err := repo.FindByID(context.Background(), "does-not-exist")

	if err != nil {
		t.Fatalf("FindByID() error = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("FindByID() = %+v, want nil for a missing document", got)
	}
}

func TestFirestoreStateRepo_MarkCompleted(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	repo := NewFirestoreStateRepo(client)
	ctx := context.Background()
	appt := newTestAppointment("mark-completed-001", "insured-mark-completed")

	if err := repo.Save(ctx, appt); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
	if err := repo.MarkCompleted(ctx, appt.AppointmentUUID); err != nil {
		t.Fatalf("MarkCompleted() error = %v, want nil", err)
	}

	got, err := repo.FindByID(ctx, appt.AppointmentUUID)
	if err != nil {
		t.Fatalf("FindByID() error = %v, want nil", err)
	}
	if got.Status != domain.StatusCompleted {
		t.Fatalf("Status = %q, want %q", got.Status, domain.StatusCompleted)
	}
}

func TestFirestoreStateRepo_ListByInsured_PaginatesWithCursor(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	repo := NewFirestoreStateRepo(client)
	ctx := context.Background()
	insuredID := "insured-list-paginated"

	for i := 0; i < 3; i++ {
		appt := newTestAppointment("list-page-"+string(rune('a'+i)), insuredID)
		appt.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
		if err := repo.Save(ctx, appt); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}
	}

	firstPage, err := repo.ListByInsured(ctx, insuredID, 2, "")
	if err != nil {
		t.Fatalf("ListByInsured() error = %v, want nil", err)
	}
	if len(firstPage.Items) != 2 {
		t.Fatalf("len(firstPage.Items) = %d, want 2", len(firstPage.Items))
	}
	if firstPage.NextCursor == nil {
		t.Fatal("firstPage.NextCursor = nil, want a cursor since more items remain")
	}

	secondPage, err := repo.ListByInsured(ctx, insuredID, 2, *firstPage.NextCursor)
	if err != nil {
		t.Fatalf("ListByInsured() with cursor error = %v, want nil", err)
	}
	if len(secondPage.Items) != 1 {
		t.Fatalf("len(secondPage.Items) = %d, want 1", len(secondPage.Items))
	}
	if secondPage.NextCursor != nil {
		t.Fatal("secondPage.NextCursor = non-nil, want nil since no items remain")
	}
}

func TestFirestoreStateRepo_ListByInsured_UnknownInsuredReturnsEmpty(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	repo := NewFirestoreStateRepo(client)

	page, err := repo.ListByInsured(context.Background(), "insured-does-not-exist", 10, "")

	if err != nil {
		t.Fatalf("ListByInsured() error = %v, want nil", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("len(page.Items) = %d, want 0", len(page.Items))
	}
	if page.NextCursor != nil {
		t.Fatal("page.NextCursor = non-nil, want nil for an empty result")
	}
}
