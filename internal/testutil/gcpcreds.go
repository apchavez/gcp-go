// Package testutil provides test-only helpers shared across the gcp-go test suite.
package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// FakeGCPCredentials writes a throwaway, freshly-generated service-account credentials
// file to a temp dir and points GOOGLE_APPLICATION_CREDENTIALS at it for the duration of
// the test. This lets GCP client constructors (firestore.NewClient, pubsub.NewClient, the
// Cloud Trace exporter) succeed without real ADC - they only validate the key format at
// construction time, never make a network call until something is actually published/read -
// so it works identically on a dev machine with real `gcloud auth application-default
// login` credentials and in CI with none at all. The key is generated fresh per test run
// and never touches disk outside t.TempDir(), so nothing resembling a real secret is
// ever committed to the repo.
func FakeGCPCredentials(t *testing.T) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("testutil: generating fake RSA key: %v", err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("testutil: marshaling fake private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	creds := map[string]string{
		"type":                        "service_account",
		"project_id":                  "fake-project-id",
		"private_key_id":              "fake-key-id",
		"private_key":                 string(keyPEM),
		"client_email":                "fake@fake-project-id.iam.gserviceaccount.com",
		"client_id":                   "000000000000000000000",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/fake%40fake-project-id.iam.gserviceaccount.com",
	}
	body, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("testutil: marshaling fake credentials JSON: %v", err)
	}

	path := filepath.Join(t.TempDir(), "fake-gcp-credentials.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("testutil: writing fake credentials file: %v", err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
}
