// Package migration embeds the Cloud SQL schema so cmd/worker can apply it on startup.
//
// There is no separate migration-runner step anywhere (not in Terraform, not in CI/CD, not
// in a one-off job) - this SQL file existed but nothing ever executed it against the live
// Cloud SQL instance, so the "appointments" table never existed. That went unnoticed because
// the worker's Pub/Sub push handler was calling the wrong service method (see
// internal/infrastructure/messaging/worker_handler.go) and never actually reached the Cloud
// SQL write path until that was fixed. Found 2026-07-27 during interview-demo E2E
// verification - this was genuinely the first time the real Cloud SQL connection got
// exercised. The DDL is idempotent (CREATE TABLE/INDEX IF NOT EXISTS), so applying it on
// every worker boot is safe and needs no separate migration tooling.
package migration

import _ "embed"

//go:embed V1__create_appointments_table.sql
var CreateAppointmentsTable string
