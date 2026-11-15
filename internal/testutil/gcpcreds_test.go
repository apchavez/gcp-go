package testutil

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFakeGCPCredentials_WritesValidServiceAccountJSON(t *testing.T) {
	FakeGCPCredentials(t)

	path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if path == "" {
		t.Fatal("GOOGLE_APPLICATION_CREDENTIALS was not set")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading credentials file: %v", err)
	}

	var creds map[string]string
	if err := json.Unmarshal(body, &creds); err != nil {
		t.Fatalf("credentials file is not valid JSON: %v", err)
	}

	if creds["type"] != "service_account" {
		t.Fatalf("type = %q, want %q", creds["type"], "service_account")
	}
	if creds["private_key"] == "" {
		t.Fatal("private_key is empty")
	}
	if creds["client_email"] == "" {
		t.Fatal("client_email is empty")
	}
}
