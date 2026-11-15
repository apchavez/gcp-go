package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apchavez/gcp-go/internal/domain"
)

func newTestNotifier(t *testing.T, handler http.HandlerFunc) *SendGridNotifier {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	n := NewSendGridNotifier("fake-api-key", "no-reply@example.com")
	n.client.BaseURL = srv.URL
	return n
}

func email(addr string) *string { return &addr }

func TestNotifyCompleted_NoOpWhenContactEmailMissing(t *testing.T) {
	called := false
	n := newTestNotifier(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	})

	err := n.NotifyCompleted(context.Background(), domain.Appointment{AppointmentUUID: "uuid-1", ContactEmail: nil})

	if err != nil {
		t.Fatalf("NotifyCompleted() error = %v, want nil", err)
	}
	if called {
		t.Fatal("NotifyCompleted() should not call SendGrid when ContactEmail is nil")
	}
}

func TestNotifyCompleted_SendsWhenContactEmailPresent(t *testing.T) {
	called := false
	n := newTestNotifier(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	})

	err := n.NotifyCompleted(context.Background(), domain.Appointment{
		AppointmentUUID: "uuid-1",
		ContactEmail:    email("patient@example.com"),
	})

	if err != nil {
		t.Fatalf("NotifyCompleted() error = %v, want nil (best-effort, never returns an error)", err)
	}
	if !called {
		t.Fatal("NotifyCompleted() should call SendGrid when ContactEmail is set")
	}
}

func TestNotifyCompleted_LogsButDoesNotErrorOnSendGridRejection(t *testing.T) {
	n := newTestNotifier(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	err := n.NotifyCompleted(context.Background(), domain.Appointment{
		AppointmentUUID: "uuid-1",
		ContactEmail:    email("patient@example.com"),
	})

	if err != nil {
		t.Fatalf("NotifyCompleted() error = %v, want nil even when SendGrid rejects the request", err)
	}
}
