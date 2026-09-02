BEGIN;

CREATE TABLE fire_inspection_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    customer_id UUID NOT NULL
        REFERENCES customers(id) ON DELETE RESTRICT,

    site_id UUID NOT NULL
        REFERENCES sites(id) ON DELETE RESTRICT,

    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT' CHECK (
        status IN (
            'DRAFT',
            'SCHEDULED',
            'IN_PROGRESS',
            'COMPLETED',
            'CANCELLED'
        )
    ),

    scheduled_for DATE,
    performed_at TIMESTAMPTZ,

    notes TEXT,

    created_by_user_id UUID
        REFERENCES users(id) ON DELETE SET NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,

    CONSTRAINT ck_fire_inspection_jobs_performed_at CHECK (
        performed_at IS NULL OR performed_at >= created_at
    )
);

CREATE TABLE fire_inspection_rows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    fire_inspection_job_id UUID NOT NULL
        REFERENCES fire_inspection_jobs(id) ON DELETE RESTRICT,

    equipment_location_id UUID NOT NULL
        REFERENCES fire_equipment_locations(id) ON DELETE RESTRICT,

    fire_extinguisher_id UUID
        REFERENCES fire_extinguishers(id) ON DELETE SET NULL,

    row_result VARCHAR(50) NOT NULL CHECK (
        row_result IN (
            'ELLENORIZVE',
            'JAVITAS',
            'UJ',
            'HIANYZIK'
        )
    ),

    okf_number VARCHAR(100),
    extinguisher_type_code VARCHAR(50),
    extinguisher_type_display VARCHAR(200),
    capacity_kg NUMERIC(8,2),

    notes TEXT,

    sort_order INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_fire_inspection_rows_capacity CHECK (
        capacity_kg IS NULL OR capacity_kg > 0
    ),

    CONSTRAINT uq_fire_inspection_rows_job_location UNIQUE (
        fire_inspection_job_id,
        equipment_location_id
    )
);

CREATE INDEX idx_fire_inspection_jobs_site_status
    ON fire_inspection_jobs (site_id, status, scheduled_for)
    WHERE archived_at IS NULL;

CREATE INDEX idx_fire_inspection_jobs_customer_status
    ON fire_inspection_jobs (customer_id, status, scheduled_for)
    WHERE archived_at IS NULL;

CREATE INDEX idx_fire_inspection_rows_job_sort
    ON fire_inspection_rows (
        fire_inspection_job_id,
        sort_order,
        equipment_location_id
    );

CREATE INDEX idx_fire_inspection_rows_location_history
    ON fire_inspection_rows (
        equipment_location_id,
        created_at DESC
    );

INSERT INTO schema_migrations (version)
VALUES ('0004_fire_inspection_jobs');

COMMIT;
