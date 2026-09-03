BEGIN;

CREATE TABLE inspectors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name VARCHAR(200) NOT NULL,
    phone VARCHAR(100),
    email VARCHAR(320),
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

CREATE INDEX inspectors_active_name_idx
    ON inspectors (name)
    WHERE archived_at IS NULL;

CREATE TABLE inspector_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    inspector_id UUID NOT NULL
        REFERENCES inspectors(id) ON DELETE RESTRICT,

    certificate_number VARCHAR(200) NOT NULL,
    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

CREATE INDEX inspector_certificates_active_inspector_idx
    ON inspector_certificates (inspector_id)
    WHERE archived_at IS NULL;

CREATE UNIQUE INDEX inspector_certificates_active_number_unique_idx
    ON inspector_certificates (certificate_number)
    WHERE archived_at IS NULL;

INSERT INTO schema_migrations (version)
VALUES ('0011_inspectors');

COMMIT;
