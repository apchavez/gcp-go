package repos

import (
	"context"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

const appointmentEventsCollection = "appointment-events"

// FirestoreEventStore implements domain.AppointmentEventStore as a separate Firestore
// collection from appointment state, mirroring the Azure sibling's separate
// "appointment-events" Cosmos container (co-located by appointmentId there via partition
// key; here via a top-level field query, since Firestore's native collection model doesn't
// have Cosmos-style partition keys). Append-only - never updated or replaced.
type FirestoreEventStore struct {
	client *firestore.Client
	res    *resilience.Resilience
}

func NewFirestoreEventStore(client *firestore.Client) *FirestoreEventStore {
	return &FirestoreEventStore{client: client, res: resilience.New("firestore-event-store")}
}

func (s *FirestoreEventStore) Append(ctx context.Context, e domain.AppointmentEvent) error {
	return s.res.Run(ctx, func() error {
		_, err := s.client.Collection(appointmentEventsCollection).Doc(e.EventID).Set(ctx, e)
		return err
	})
}

func (s *FirestoreEventStore) FindByAppointmentID(ctx context.Context, appointmentUUID string) ([]domain.AppointmentEvent, error) {
	var events []domain.AppointmentEvent
	err := s.res.Run(ctx, func() error {
		iter := s.client.Collection(appointmentEventsCollection).
			Where("appointmentUuid", "==", appointmentUUID).
			OrderBy("occurredAt", firestore.Asc).
			Documents(ctx)
		defer iter.Stop()

		events = nil
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			var e domain.AppointmentEvent
			if err := doc.DataTo(&e); err != nil {
				return err
			}
			events = append(events, e)
		}
		return nil
	})
	return events, err
}
