package repos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

const appointmentsCollection = "appointments"

// FirestoreStateRepo implements domain.AppointmentStateRepository against a single
// Firestore collection, mirroring the AWS sibling's DynamoDB table / Azure sibling's
// Cosmos "appointments" container (partition key /appointmentUuid there, document ID here).
type FirestoreStateRepo struct {
	client *firestore.Client
	res    *resilience.Resilience
}

func NewFirestoreStateRepo(client *firestore.Client) *FirestoreStateRepo {
	return &FirestoreStateRepo{client: client, res: resilience.New("firestore-state-repo")}
}

func (r *FirestoreStateRepo) Save(ctx context.Context, a domain.Appointment) error {
	return r.res.Run(ctx, func() error {
		_, err := r.client.Collection(appointmentsCollection).Doc(a.AppointmentUUID).Set(ctx, a)
		return err
	})
}

func (r *FirestoreStateRepo) FindByID(ctx context.Context, appointmentUUID string) (*domain.Appointment, error) {
	var appointment domain.Appointment
	err := r.res.Run(ctx, func() error {
		doc, err := r.client.Collection(appointmentsCollection).Doc(appointmentUUID).Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil
			}
			return err
		}
		return doc.DataTo(&appointment)
	})
	if err != nil {
		return nil, err
	}
	if appointment.AppointmentUUID == "" {
		return nil, nil
	}
	return &appointment, nil
}

func (r *FirestoreStateRepo) updateStatus(ctx context.Context, appointmentUUID, status string) error {
	return r.res.Run(ctx, func() error {
		_, err := r.client.Collection(appointmentsCollection).Doc(appointmentUUID).Update(ctx, []firestore.Update{
			{Path: "status", Value: status},
		})
		return err
	})
}

func (r *FirestoreStateRepo) MarkCompleted(ctx context.Context, appointmentUUID string) error {
	return r.updateStatus(ctx, appointmentUUID, domain.StatusCompleted)
}

func (r *FirestoreStateRepo) MarkCancelled(ctx context.Context, appointmentUUID string) error {
	return r.updateStatus(ctx, appointmentUUID, domain.StatusCancelled)
}

func (r *FirestoreStateRepo) MarkRescheduled(ctx context.Context, appointmentUUID string) error {
	return r.updateStatus(ctx, appointmentUUID, domain.StatusRescheduled)
}

// cursor is an opaque base64url-encoded JSON blob wrapping the last document's Firestore
// document ID, mirroring the AWS sibling's DynamoDB LastEvaluatedKey-based cursor pattern.
type cursorPayload struct {
	LastDocID string `json:"lastDocId"`
}

func encodeCursor(docID string) string {
	b, _ := json.Marshal(cursorPayload{LastDocID: docID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(cursor string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	var p cursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return "", err
	}
	return p.LastDocID, nil
}

func (r *FirestoreStateRepo) ListByInsured(ctx context.Context, insuredID string, pageSize int, cursor string) (domain.Page, error) {
	var page domain.Page
	err := r.res.Run(ctx, func() error {
		query := r.client.Collection(appointmentsCollection).
			Where("insuredId", "==", insuredID).
			OrderBy("createdAt", firestore.Asc).
			Limit(pageSize)

		if cursor != "" {
			lastDocID, err := decodeCursor(cursor)
			if err != nil {
				return fmt.Errorf("invalid cursor: %w", err)
			}
			snap, err := r.client.Collection(appointmentsCollection).Doc(lastDocID).Get(ctx)
			if err != nil {
				return err
			}
			query = query.StartAfter(snap)
		}

		iter := query.Documents(ctx)
		defer iter.Stop()

		var items []domain.Appointment
		var lastDocID string
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			var a domain.Appointment
			if err := doc.DataTo(&a); err != nil {
				return err
			}
			items = append(items, a)
			lastDocID = doc.Ref.ID
		}

		page.Items = items
		if len(items) == pageSize {
			next := encodeCursor(lastDocID)
			page.NextCursor = &next
		}
		return nil
	})
	return page, err
}
