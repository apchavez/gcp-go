package repos

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

// CloudSQLRepo implements domain.AppointmentRelationalRepository - the relational side
// that only ProcessAppointment writes to (final/completed appointments), mirroring the
// AWS sibling's RDS MySQL and the Azure sibling's Azure SQL "appointments" table.
type CloudSQLRepo struct {
	pool *pgxpool.Pool
	res  *resilience.Resilience
}

func NewCloudSQLRepo(pool *pgxpool.Pool) *CloudSQLRepo {
	return &CloudSQLRepo{pool: pool, res: resilience.New("cloudsql-repo")}
}

const upsertSQL = `
INSERT INTO appointments (appointment_uuid, insured_id, schedule_id, country_iso, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (appointment_uuid) DO UPDATE SET
	status = EXCLUDED.status,
	updated_at = EXCLUDED.updated_at
`

func (r *CloudSQLRepo) Persist(ctx context.Context, a domain.Appointment) error {
	return r.res.Run(ctx, func() error {
		_, err := r.pool.Exec(ctx, upsertSQL,
			a.AppointmentUUID, a.InsuredID, a.ScheduleID, a.CountryISO, a.Status, a.CreatedAt, a.UpdatedAt,
		)
		return err
	})
}
