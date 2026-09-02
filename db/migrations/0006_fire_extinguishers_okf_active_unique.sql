BEGIN;

DROP INDEX IF EXISTS uq_fire_extinguishers_okf_number_active;

CREATE UNIQUE INDEX uq_fire_extinguishers_okf_number_active
ON fire_extinguishers (okf_number)
WHERE okf_number IS NOT NULL
  AND archived_at IS NULL
  AND lifecycle_status = 'ACTIVE_AT_CUSTOMER';

COMMIT;
