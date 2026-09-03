BEGIN;

CREATE TABLE customer_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    customer_id UUID NOT NULL
        REFERENCES customers(id),

    site_id UUID
        REFERENCES sites(id),

    name VARCHAR(200) NOT NULL,
    role_title VARCHAR(160),

    phone VARCHAR(100),
    email VARCHAR(320),

    preferred_contact_method VARCHAR(20),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,

    notes TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,

    CONSTRAINT ck_customer_contacts_preferred_contact_method
    CHECK (
        preferred_contact_method IS NULL
        OR preferred_contact_method IN (
            'PHONE',
            'EMAIL',
            'PERSONAL',
            'OTHER'
        )
    )
);

CREATE INDEX customer_contacts_customer_idx
    ON customer_contacts (customer_id)
    WHERE archived_at IS NULL;

CREATE INDEX customer_contacts_site_idx
    ON customer_contacts (site_id)
    WHERE archived_at IS NULL;

CREATE INDEX customer_contacts_search_idx
    ON customer_contacts (customer_id, name)
    WHERE archived_at IS NULL;

CREATE UNIQUE INDEX customer_contacts_one_primary_per_customer_idx
    ON customer_contacts (customer_id)
    WHERE is_primary = TRUE
      AND site_id IS NULL
      AND archived_at IS NULL;

CREATE UNIQUE INDEX customer_contacts_one_primary_per_site_idx
    ON customer_contacts (site_id)
    WHERE is_primary = TRUE
      AND archived_at IS NULL;

INSERT INTO schema_migrations (version)
VALUES ('0010_customer_contacts');

COMMIT;
