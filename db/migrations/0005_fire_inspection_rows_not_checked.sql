BEGIN;

ALTER TABLE fire_inspection_rows
    DROP CONSTRAINT IF EXISTS fire_inspection_rows_row_result_check;

ALTER TABLE fire_inspection_rows
    ADD CONSTRAINT fire_inspection_rows_row_result_check
    CHECK (
        row_result IN (
            'NEM_ELLENORIZVE',
            'ELLENORIZVE',
            'JAVITAS',
            'UJ',
            'HIANYZIK'
        )
    );

COMMIT;
