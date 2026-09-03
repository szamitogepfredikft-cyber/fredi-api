package fireinspectionrows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RowResultNotChecked = "NEM_ELLENORIZVE"
	RowResultChecked    = "ELLENORIZVE"
	RowResultRepair     = "JAVITAS"
	RowResultNew        = "UJ"
	RowResultMissing    = "HIANYZIK"
)

var (
	ErrJobNotFound           = errors.New("fire inspection job not found")
	ErrJobNotEditable        = errors.New("fire inspection job is not editable")
	ErrRowNotFound           = errors.New("fire inspection row not found")
	ErrRowDoesNotBelongToJob = errors.New("fire inspection row does not belong to job")
	ErrInvalidRowResult      = errors.New("invalid fire inspection row result")
	ErrInvalidCapacity       = errors.New("capacity_kg must be greater than zero")
)

type Row struct {
	ID                      uuid.UUID  `json:"id"`
	FireInspectionJobID     uuid.UUID  `json:"fire_inspection_job_id"`
	EquipmentLocationID     uuid.UUID  `json:"equipment_location_id"`
	EquipmentLocationCode   *string    `json:"equipment_location_code,omitempty"`
	EquipmentLocationName   string     `json:"equipment_location_name"`
	EquipmentLocationZone   *string    `json:"equipment_location_zone,omitempty"`
	FireExtinguisherID      *uuid.UUID `json:"fire_extinguisher_id,omitempty"`
	RowResult               string     `json:"row_result"`
	OKFNumber               *string    `json:"okf_number,omitempty"`
	ExtinguisherTypeCode    *string    `json:"extinguisher_type_code,omitempty"`
	ExtinguisherTypeDisplay *string    `json:"extinguisher_type_display,omitempty"`
	CapacityKG              *float64   `json:"capacity_kg,omitempty"`
	InspectionQuarter       *string    `json:"inspection_quarter,omitempty"`
	Notes                   *string    `json:"notes,omitempty"`
	SortOrder               int        `json:"sort_order"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type UpdateInput struct {
	RowResult               string
	OKFNumber               optionalString
	ExtinguisherTypeCode    optionalString
	ExtinguisherTypeDisplay optionalString
	CapacityKG              optionalFloat64
	Notes                   optionalString
}

type optionalString struct {
	Set   bool
	Value *string
}

type optionalFloat64 struct {
	Set   bool
	Value *float64
}

func (i *UpdateInput) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	allowedFields := map[string]struct{}{
		"row_result":                {},
		"okf_number":                {},
		"extinguisher_type_code":    {},
		"extinguisher_type_display": {},
		"capacity_kg":               {},
		"notes":                     {},
	}

	for field := range raw {
		if _, ok := allowedFields[field]; !ok {
			return errors.New("unknown JSON field")
		}
	}

	rowResultRaw, ok := raw["row_result"]
	if !ok {
		return errors.New("row_result is required")
	}

	if err := json.Unmarshal(rowResultRaw, &i.RowResult); err != nil {
		return errors.New("row_result must be a string")
	}

	if err := decodeOptionalString(raw, "okf_number", &i.OKFNumber); err != nil {
		return err
	}

	if err := decodeOptionalString(
		raw,
		"extinguisher_type_code",
		&i.ExtinguisherTypeCode,
	); err != nil {
		return err
	}

	if err := decodeOptionalString(
		raw,
		"extinguisher_type_display",
		&i.ExtinguisherTypeDisplay,
	); err != nil {
		return err
	}

	if err := decodeOptionalFloat64(raw, "capacity_kg", &i.CapacityKG); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "notes", &i.Notes); err != nil {
		return err
	}

	return nil
}

func decodeOptionalString(
	raw map[string]json.RawMessage,
	field string,
	target *optionalString,
) error {
	value, ok := raw[field]
	if !ok {
		return nil
	}

	target.Set = true

	if bytes.Equal(value, []byte("null")) {
		target.Value = nil
		return nil
	}

	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return errors.New(field + " must be a string or null")
	}

	target.Value = &decoded
	return nil
}

func decodeOptionalFloat64(
	raw map[string]json.RawMessage,
	field string,
	target *optionalFloat64,
) error {
	value, ok := raw[field]
	if !ok {
		return nil
	}

	target.Set = true

	if bytes.Equal(value, []byte("null")) {
		target.Value = nil
		return nil
	}

	var decoded float64
	if err := json.Unmarshal(value, &decoded); err != nil {
		return errors.New(field + " must be a number or null")
	}

	target.Value = &decoded
	return nil
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InitializeForJob(
	ctx context.Context,
	jobID uuid.UUID,
) ([]Row, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var jobExists bool

	err = tx.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM fire_inspection_jobs j
			JOIN customers c
				ON c.id = j.customer_id
			   AND c.archived_at IS NULL
			JOIN sites s
				ON s.id = j.site_id
			   AND s.customer_id = j.customer_id
			   AND s.archived_at IS NULL
			WHERE j.id = $1
			  AND j.archived_at IS NULL
		)
		`,
		jobID,
	).Scan(&jobExists)
	if err != nil {
		return nil, false, err
	}

	if !jobExists {
		return nil, false, ErrJobNotFound
	}

	var existingRowCount int

	err = tx.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM fire_inspection_rows
		WHERE fire_inspection_job_id = $1
		`,
		jobID,
	).Scan(&existingRowCount)
	if err != nil {
		return nil, false, err
	}

	const initializeQuery = `
		INSERT INTO fire_inspection_rows (
			fire_inspection_job_id,
			equipment_location_id,
			fire_extinguisher_id,
			row_result,
			okf_number,
			extinguisher_type_code,
			extinguisher_type_display,
			capacity_kg,
			sort_order,
                        inspection_quarter
		)
		SELECT
			j.id,
			l.id,
			e.id,
			$2,
			e.okf_number,
			e.extinguisher_type_code,
			e.extinguisher_type_display,
			e.capacity_kg,
			l.sort_order,
		        j.inspection_quarter
                FROM fire_inspection_jobs j
		JOIN customers c
			ON c.id = j.customer_id
		   AND c.archived_at IS NULL
		JOIN sites s
			ON s.id = j.site_id
		   AND s.customer_id = j.customer_id
		   AND s.archived_at IS NULL
		JOIN fire_equipment_locations l
			ON l.site_id = j.site_id
		   AND l.archived_at IS NULL
		LEFT JOIN extinguisher_location_assignments a
			ON a.equipment_location_id = l.id
		   AND a.unassigned_at IS NULL
		LEFT JOIN fire_extinguishers e
			ON e.id = a.fire_extinguisher_id
		   AND e.archived_at IS NULL
		WHERE j.id = $1
		  AND j.archived_at IS NULL
		ON CONFLICT (fire_inspection_job_id, equipment_location_id) DO NOTHING
	`

	_, err = tx.Exec(ctx, initializeQuery, jobID, RowResultNotChecked)
	if err != nil {
		return nil, false, err
	}

	rows, err := listByJobID(ctx, tx, jobID)
	if err != nil {
		return nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}

	return rows, existingRowCount == 0, nil
}

func (r *Repository) ListByJobID(
	ctx context.Context,
	jobID uuid.UUID,
) ([]Row, error) {
	var jobExists bool

	err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM fire_inspection_jobs
			WHERE id = $1
			  AND archived_at IS NULL
		)
		`,
		jobID,
	).Scan(&jobExists)
	if err != nil {
		return nil, err
	}

	if !jobExists {
		return nil, ErrJobNotFound
	}

	return listByJobID(ctx, r.db, jobID)
}

func (r *Repository) Update(
	ctx context.Context,
	jobID uuid.UUID,
	rowID uuid.UUID,
	input UpdateInput,
) (Row, error) {
	result := strings.TrimSpace(input.RowResult)

	if !isUpdatableRowResult(result) {
		return Row{}, ErrInvalidRowResult
	}

	if input.CapacityKG.Set &&
		input.CapacityKG.Value != nil &&
		*input.CapacityKG.Value <= 0 {
		return Row{}, ErrInvalidCapacity
	}

	var jobStatus string

	err := r.db.QueryRow(
		ctx,
		`
		SELECT j.status
		FROM fire_inspection_jobs j
		JOIN customers c
			ON c.id = j.customer_id
		   AND c.archived_at IS NULL
		JOIN sites s
			ON s.id = j.site_id
		   AND s.customer_id = j.customer_id
		   AND s.archived_at IS NULL
		WHERE j.id = $1
		  AND j.archived_at IS NULL
		`,
		jobID,
	).Scan(&jobStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, ErrJobNotFound
	}
	if err != nil {
		return Row{}, err
	}

	if jobStatus == "COMPLETED" || jobStatus == "CANCELLED" {
		return Row{}, ErrJobNotEditable
	}

	var actualJobID uuid.UUID

	err = r.db.QueryRow(
		ctx,
		`
		SELECT fire_inspection_job_id
		FROM fire_inspection_rows
		WHERE id = $1
		`,
		rowID,
	).Scan(&actualJobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, ErrRowNotFound
	}
	if err != nil {
		return Row{}, err
	}

	if actualJobID != jobID {
		return Row{}, ErrRowDoesNotBelongToJob
	}

	const updateQuery = `
		UPDATE fire_inspection_rows
		SET
			row_result = $3,
			fire_extinguisher_id = CASE
				WHEN $4 THEN NULL
				ELSE fire_extinguisher_id
			END,
			okf_number = CASE
				WHEN $4 THEN NULL
				WHEN $5 THEN $6
				ELSE okf_number
			END,
			extinguisher_type_code = CASE
				WHEN $4 THEN NULL
				WHEN $7 THEN $8
				ELSE extinguisher_type_code
			END,
			extinguisher_type_display = CASE
				WHEN $4 THEN NULL
				WHEN $9 THEN $10
				ELSE extinguisher_type_display
			END,
			capacity_kg = CASE
				WHEN $4 THEN NULL
				WHEN $11 THEN $12
				ELSE capacity_kg
			END,
			notes = CASE
				WHEN $13 THEN $14
				ELSE notes
			END,
			updated_at = now()
		WHERE id = $1
		  AND fire_inspection_job_id = $2
	`

	missing := result == RowResultMissing

	_, err = r.db.Exec(
		ctx,
		updateQuery,
		rowID,
		jobID,
		result,
		missing,
		input.OKFNumber.Set,
		normalizedOptionalString(input.OKFNumber.Value),
		input.ExtinguisherTypeCode.Set,
		normalizedOptionalString(input.ExtinguisherTypeCode.Value),
		input.ExtinguisherTypeDisplay.Set,
		normalizedOptionalString(input.ExtinguisherTypeDisplay.Value),
		input.CapacityKG.Set,
		input.CapacityKG.Value,
		input.Notes.Set,
		normalizedOptionalString(input.Notes.Value),
	)
	if err != nil {
		return Row{}, err
	}

	return r.GetByID(ctx, jobID, rowID)
}

func (r *Repository) GetByID(
	ctx context.Context,
	jobID uuid.UUID,
	rowID uuid.UUID,
) (Row, error) {
	const query = `
		SELECT
			r.id,
			r.fire_inspection_job_id,
			r.equipment_location_id,
			l.location_code,
			l.description,
			l.floor_or_zone,
			r.fire_extinguisher_id,
			r.row_result,
			r.okf_number,
			r.extinguisher_type_code,
			r.extinguisher_type_display,
			r.capacity_kg,
			r.inspection_quarter,
                        r.notes,
			r.sort_order,
			r.created_at,
			r.updated_at
		FROM fire_inspection_rows r
		JOIN fire_equipment_locations l
			ON l.id = r.equipment_location_id
		WHERE r.id = $1
		  AND r.fire_inspection_job_id = $2
	`

	var row Row

	err := r.db.QueryRow(ctx, query, rowID, jobID).Scan(
		&row.ID,
		&row.FireInspectionJobID,
		&row.EquipmentLocationID,
		&row.EquipmentLocationCode,
		&row.EquipmentLocationName,
		&row.EquipmentLocationZone,
		&row.FireExtinguisherID,
		&row.RowResult,
		&row.OKFNumber,
		&row.ExtinguisherTypeCode,
		&row.ExtinguisherTypeDisplay,
		&row.CapacityKG,
		&row.InspectionQuarter,
		&row.Notes,
		&row.SortOrder,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, ErrRowNotFound
	}
	if err != nil {
		return Row{}, err
	}

	row.EquipmentLocationName = strings.TrimSpace(row.EquipmentLocationName)

	return row, nil
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func listByJobID(
	ctx context.Context,
	q queryer,
	jobID uuid.UUID,
) ([]Row, error) {
	const query = `
		SELECT
			r.id,
			r.fire_inspection_job_id,
			r.equipment_location_id,
			l.location_code,
			l.description,
			l.floor_or_zone,
			r.fire_extinguisher_id,
			r.row_result,
			r.okf_number,
			r.extinguisher_type_code,
			r.extinguisher_type_display,
			r.capacity_kg,
			r.inspection_quarter,
                        r.notes,
			r.sort_order,
			r.created_at,
			r.updated_at
		FROM fire_inspection_rows r
		JOIN fire_equipment_locations l
			ON l.id = r.equipment_location_id
		WHERE r.fire_inspection_job_id = $1
		ORDER BY r.sort_order ASC, l.description ASC, r.id ASC
	`

	dbRows, err := q.Query(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	rowList := make([]Row, 0)

	for dbRows.Next() {
		var row Row

		err := dbRows.Scan(
			&row.ID,
			&row.FireInspectionJobID,
			&row.EquipmentLocationID,
			&row.EquipmentLocationCode,
			&row.EquipmentLocationName,
			&row.EquipmentLocationZone,
			&row.FireExtinguisherID,
			&row.RowResult,
			&row.OKFNumber,
			&row.ExtinguisherTypeCode,
			&row.ExtinguisherTypeDisplay,
			&row.CapacityKG,
			&row.InspectionQuarter,
			&row.Notes,
			&row.SortOrder,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		row.EquipmentLocationName = strings.TrimSpace(row.EquipmentLocationName)
		rowList = append(rowList, row)
	}

	if err := dbRows.Err(); err != nil {
		return nil, err
	}

	return rowList, nil
}

func isUpdatableRowResult(value string) bool {
	switch value {
	case RowResultChecked, RowResultRepair, RowResultMissing:
		return true
	default:
		return false
	}
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
