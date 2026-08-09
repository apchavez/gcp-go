package main

import (
	"strings"
	"testing"
)

func TestCloudSQLDSN_BuildsPostgresConnectionString(t *testing.T) {
	t.Setenv("CLOUDSQL_SOCKET_DIR", "/cloudsql/proj:region:instance")
	t.Setenv("CLOUDSQL_USER", "app-user")
	t.Setenv("CLOUDSQL_DB", "appointments")
	t.Setenv("CLOUDSQL_PASSWORD", "p@ss/word")

	dsn := cloudSQLDSN()

	if !strings.HasPrefix(dsn, "postgres://app-user:") {
		t.Fatalf("cloudSQLDSN() = %q, want prefix %q", dsn, "postgres://app-user:")
	}
	if !strings.Contains(dsn, "host=%2Fcloudsql%2Fproj%3Aregion%3Ainstance") {
		t.Fatalf("cloudSQLDSN() = %q, want it to contain the escaped socket dir as host", dsn)
	}
	if !strings.Contains(dsn, "/appointments?") {
		t.Fatalf("cloudSQLDSN() = %q, want it to contain the db name", dsn)
	}
}
