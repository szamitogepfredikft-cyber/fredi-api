BEGIN;
CREATE TABLE fire_inspection_due_dates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    customer_id UUID NOT NULL
        REFERENCES customers(id),

    site_id UUID NOT NULL
        REFERENCES sites(id),

    source_fire_inspection_job_id UUID
        REFERENCES fire_inspection_jobs(id),

    fire_inspection_job_id UUID
        REFERENCES fire_inspection_jobs(id),

    due_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'TERVEZETT',

    due_date DATE NOT NULL,
    coverage_year INTEGER,

    contact_next_at DATE,
    scheduled_for DATE,

    notes TEXT,
    reschedule_reason TEXT,

    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fire_inspection_due_dates_type_check CHECK (
        due_type IN (
            'ANNUAL_INSPECTION',
            'MAINTENANCE_5_YEAR',
            'MAINTENANCE_10_YEAR',
            'MAINTENANCE_15_YEAR',
            'REPAIR_FOLLOW_UP',
            'CUSTOM'
        )
    ),

    CONSTRAINT fire_inspection_due_dates_status_check CHECK (
        status IN (
            'TERVEZETT',
            'AKTUALIS',
            'KAPCSOLATFELVETELRE_VAR',
            'EGYEZTETES_ALATT',
            'IDOPONT_EGYEZTETVE',
            'MUNKALAP_LETREHOZVA',
            'ELVEGEZVE',
            'ATUTEMEZVE',
            'TOROLVE'
        )
    ),

    CONSTRAINT fire_inspection_due_dates_customer_site_check CHECK (
        customer_id IS NOT NULL
        AND site_id IS NOT NULL
    )
);

CREATE INDEX fire_inspection_due_dates_active_due_date_idx
    ON fire_inspection_due_dates (due_date)
    WHERE archived_at IS NULL
      AND status <> 'TOROLVE';

CREATE INDEX fire_inspection_due_dates_customer_site_idx
    ON fire_inspection_due_dates (customer_id, site_id)
    WHERE archived_at IS NULL;

CREATE UNIQUE INDEX fire_inspection_due_dates_annual_coverage_idx
    ON fire_inspection_due_dates (
        customer_id,
        site_id,
        due_type,
        coverage_year
    )
    WHERE due_type = 'ANNUAL_INSPECTION'
      AND coverage_year IS NOT NULL
      AND archived_at IS NULL
      AND status <> 'TOROLVE';
INSERT INTO schema_migrations (version)
VALUES ('0008_fire_inspection_due_dates');

COMMIT;
