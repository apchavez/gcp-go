# Firestore Native mode database for the "appointments" state store and the separate
# "appointment-events" event-sourcing collection. Firestore collections are created
# implicitly on first write - only the database itself and its composite indexes are
# provisioned here.

resource "google_firestore_database" "default" {
  project     = var.gcp_project_id
  name        = "(default)"
  location_id = var.gcp_region
  type        = "FIRESTORE_NATIVE"
}

# Supports the "appointments" collection's ListByInsured query: filter by insuredId,
# order by createdAt.
resource "google_firestore_index" "appointments_by_insured" {
  project    = var.gcp_project_id
  database   = google_firestore_database.default.name
  collection = "appointments"

  fields {
    field_path = "insuredId"
    order      = "ASCENDING"
  }
  fields {
    field_path = "createdAt"
    order      = "ASCENDING"
  }
}

# Supports the "appointment-events" collection's FindByAppointmentID query: filter by
# appointmentUuid, order by occurredAt.
resource "google_firestore_index" "events_by_appointment" {
  project    = var.gcp_project_id
  database   = google_firestore_database.default.name
  collection = "appointment-events"

  fields {
    field_path = "appointmentUuid"
    order      = "ASCENDING"
  }
  fields {
    field_path = "occurredAt"
    order      = "ASCENDING"
  }
}
