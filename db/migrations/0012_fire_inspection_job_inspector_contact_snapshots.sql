BEGIN;

ALTER TABLE fire_inspection_jobs
    ADD COLUMN inspector_phone_snapshot VARCHAR(100),
    ADD COLUMN inspector_email_snapshot VARCHAR(320);

INSERT INTO schema_migrations (version)
VALUES ('0012_fire_inspection_job_inspector_contact_snapshots');

COMMIT;
