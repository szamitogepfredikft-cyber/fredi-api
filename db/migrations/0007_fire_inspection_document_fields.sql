BEGIN;

ALTER TABLE fire_inspection_jobs
    ADD COLUMN inspection_year INTEGER,
    ADD COLUMN inspection_quarter VARCHAR(2),

    ADD COLUMN issued_at TIMESTAMPTZ,
    ADD COLUMN issued_by_user_id UUID
        REFERENCES users(id) ON DELETE SET NULL,

    ADD COLUMN issued_by_name_snapshot VARCHAR(200),
    ADD COLUMN issued_by_company_snapshot VARCHAR(200),
    ADD COLUMN issued_by_phone_snapshot VARCHAR(100),
    ADD COLUMN issued_by_email_snapshot VARCHAR(320),

    ADD COLUMN inspector_name_snapshot VARCHAR(200),
    ADD COLUMN inspector_certificate_snapshot TEXT,

    ADD COLUMN repairer_name_snapshot VARCHAR(200);

ALTER TABLE fire_inspection_jobs
    ADD CONSTRAINT ck_fire_inspection_jobs_inspection_year
    CHECK (
        inspection_year IS NULL
        OR inspection_year BETWEEN 2000 AND 2100
    );

ALTER TABLE fire_inspection_jobs
    ADD CONSTRAINT ck_fire_inspection_jobs_inspection_quarter
    CHECK (
        inspection_quarter IS NULL
        OR inspection_quarter IN ('Q1', 'Q2', 'Q3', 'Q4')
    );

ALTER TABLE fire_inspection_rows
    ADD COLUMN inspection_quarter VARCHAR(2);

ALTER TABLE fire_inspection_rows
    ADD CONSTRAINT ck_fire_inspection_rows_inspection_quarter
    CHECK (
        inspection_quarter IS NULL
        OR inspection_quarter IN ('Q1', 'Q2', 'Q3', 'Q4')
    );

INSERT INTO schema_migrations (version)
VALUES ('0007_fire_inspection_document_fields');

COMMIT;
