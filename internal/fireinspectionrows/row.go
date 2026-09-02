package fireinspectionrows

import (
	"context"
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

var ErrJobNotFound = errors.New("fire inspection job not found")

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
	Notes                   *string    `json:"notes,omitempty"`
	SortOrder               int        `json:"sort_order"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
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
			sort_order
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
			l.sort_order
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
