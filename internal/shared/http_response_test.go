package shared

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decodeMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body messageBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body.Message
}

func TestSuccessResponses_WriteStatusAndJSONBody(t *testing.T) {
	cases := []struct {
		name   string
		write  func(http.ResponseWriter, any)
		status int
	}{
		{"OK", OK, http.StatusOK},
		{"Created", Created, http.StatusCreated},
		{"Accepted", Accepted, http.StatusAccepted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			tc.write(rec, map[string]string{"id": "42"})

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			var got map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got["id"] != "42" {
				t.Fatalf("body id = %q, want 42", got["id"])
			}
		})
	}
}

func TestErrorResponses_UseGivenMessage(t *testing.T) {
	cases := []struct {
		name   string
		write  func(http.ResponseWriter, string)
		status int
	}{
		{"Bad", Bad, http.StatusBadRequest},
		{"Forbidden", Forbidden, http.StatusForbidden},
		{"NotFound", NotFound, http.StatusNotFound},
		{"Conflict", Conflict, http.StatusConflict},
		{"Internal", Internal, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			tc.write(rec, "custom message")

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			if msg := decodeMessage(t, rec); msg != "custom message" {
				t.Fatalf("message = %q, want %q", msg, "custom message")
			}
		})
	}
}

func TestErrorResponses_FallBackToDefaultMessage(t *testing.T) {
	cases := []struct {
		name  string
		write func(http.ResponseWriter, string)
		want  string
	}{
		{"Forbidden", Forbidden, "Access denied"},
		{"NotFound", NotFound, "Not found"},
		{"Internal", Internal, "Internal server error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			tc.write(rec, "")

			if msg := decodeMessage(t, rec); msg != tc.want {
				t.Fatalf("message = %q, want %q", msg, tc.want)
			}
		})
	}
}
