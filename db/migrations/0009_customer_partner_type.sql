BEGIN;

ALTER TABLE customers
    ADD COLUMN partner_type VARCHAR(32) NOT NULL DEFAULT 'COMPANY';

ALTER TABLE customers
    ADD CONSTRAINT ck_customers_partner_type
    CHECK (
        partner_type IN (
            'COMPANY',
            'SOLE_PROPRIETOR',
            'INDIVIDUAL'
        )
    );

CREATE INDEX customers_partner_type_idx
    ON customers (partner_type)
    WHERE archived_at IS NULL;

INSERT INTO schema_migrations (version)
VALUES ('0009_customer_partner_type');

COMMIT;
