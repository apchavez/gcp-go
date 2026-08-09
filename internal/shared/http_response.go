package shared

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func OK(w http.ResponseWriter, body any)       { writeJSON(w, http.StatusOK, body) }
func Created(w http.ResponseWriter, body any)  { writeJSON(w, http.StatusCreated, body) }
func Accepted(w http.ResponseWriter, body any) { writeJSON(w, http.StatusAccepted, body) }

type messageBody struct {
	Message string `json:"message"`
}

func Bad(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, messageBody{message})
}

func Forbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Access denied"
	}
	writeJSON(w, http.StatusForbidden, messageBody{message})
}

func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Not found"
	}
	writeJSON(w, http.StatusNotFound, messageBody{message})
}

func Conflict(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusConflict, messageBody{message})
}

func Internal(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Internal server error"
	}
	writeJSON(w, http.StatusInternalServerError, messageBody{message})
}
