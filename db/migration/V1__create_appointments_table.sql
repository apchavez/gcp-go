CREATE TABLE IF NOT EXISTS appointments (
    appointment_uuid VARCHAR(50)  NOT NULL,
    insured_id       VARCHAR(20)  NOT NULL,
    schedule_id      INT          NOT NULL,
    country_iso      VARCHAR(2)   NOT NULL,
    status           VARCHAR(20)  NOT NULL,
    created_at       TIMESTAMPTZ  NULL,
    updated_at       TIMESTAMPTZ  NULL,
    CONSTRAINT pk_appointments PRIMARY KEY (appointment_uuid)
);

CREATE INDEX IF NOT EXISTS ix_appointments_insured_id ON appointments (insured_id);
