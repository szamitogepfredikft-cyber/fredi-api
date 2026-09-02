package extinguisherassignments

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/extinguishers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	AssignmentReasonInitialAssignment = "INITIAL_ASSIGNMENT"
	AssignmentReasonReplacement       = "REPLACEMENT"
	AssignmentReasonReinstallation    = "REINSTALLATION"
	AssignmentReasonMigratedHistory   = "MIGRATED_HISTORY"
	AssignmentReasonOther             = "OTHER"
)

var (
	ErrExtinguisherNotFound         = errors.New("extinguisher not found")
	ErrEquipmentLocationNotFound    = errors.New("equipment location not found")
	ErrExtinguisherNotIssuable      = errors.New("extinguisher is not issuable")
	ErrExtinguisherAlreadyAssigned  = errors.New("extinguisher already has an active assignment")
	ErrEquipmentLocationAlreadyUsed = errors.New("equipment location already has an active assignment")
)

var validAssignmentReasons = map[string]struct{}{
	AssignmentReasonInitialAssignment: {},
	AssignmentReasonReplacement:       {},
	AssignmentReasonReinstallation:    {},
	AssignmentReasonMigratedHistory:   {},
	AssignmentReasonOther:             {},
}

type Assignment struct {
	ID                  uuid.UUID  `json:"id"`
	EquipmentLocationID uuid.UUID  `json:"equipment_location_id"`
	FireExtinguisherID  uuid.UUID  `json:"fire_extinguisher_id"`
	AssignedAt          time.Time  `json:"assigned_at"`
	UnassignedAt        *time.Time `json:"unassigned_at,omitempty"`
	AssignmentReason    string     `json:"assignment_reason"`
	UnassignmentReason  *string    `json:"unassignment_reason,omitempty"`
	Notes               *string    `json:"notes,omitempty"`
	CreatedByUserID     *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

type CreateInput struct {
	EquipmentLocationID uuid.UUID `json:"equipment_location_id"`
	AssignmentReason    string    `json:"assignment_reason"`
	Notes               *string   `json:"notes"`
}

type Repository struct {
	db *pgxpool.Pool
}

type rowScanner interface {
	Scan(dest ...any) error
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func IsValidAssignmentReason(value string) bool {
	_, ok := validAssignmentReasons[strings.TrimSpace(value)]
	return ok
}

func (r *Repository) Create(
	ctx context.Context,
	extinguisherID uuid.UUID,
	input CreateInput,
) (Assignment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Assignment{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var lifecycleStatus string

	err = tx.QueryRow(
		ctx,
		`
		SELECT lifecycle_status
		FROM fire_extinguishers
		WHERE id = $1
		  AND archived_at IS NULL
		FOR UPDATE
		`,
		extinguisherID,
	).Scan(&lifecycleStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, ErrExtinguisherNotFound
	}
	if err != nil {
		return Assignment{}, err
	}

	if !isIssuableLifecycleStatus(lifecycleStatus) {
		return Assignment{}, ErrExtinguisherNotIssuable
	}

	var equipmentLocationID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT id
		FROM fire_equipment_locations
		WHERE id = $1
		  AND archived_at IS NULL
		FOR UPDATE
		`,
		input.EquipmentLocationID,
	).Scan(&equipmentLocationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, ErrEquipmentLocationNotFound
	}
	if err != nil {
		return Assignment{}, err
	}

	var extinguisherAlreadyAssigned bool

	err = tx.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM extinguisher_location_assignments
			WHERE fire_extinguisher_id = $1
			  AND unassigned_at IS NULL
		)
		`,
		extinguisherID,
	).Scan(&extinguisherAlreadyAssigned)
	if err != nil {
		return Assignment{}, err
	}

	if extinguisherAlreadyAssigned {
		return Assignment{}, ErrExtinguisherAlreadyAssigned
	}

	var equipmentLocationAlreadyUsed bool

	err = tx.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM extinguisher_location_assignments
			WHERE equipment_location_id = $1
			  AND unassigned_at IS NULL
		)
		`,
		input.EquipmentLocationID,
	).Scan(&equipmentLocationAlreadyUsed)
	if err != nil {
		return Assignment{}, err
	}

	if equipmentLocationAlreadyUsed {
		return Assignment{}, ErrEquipmentLocationAlreadyUsed
	}

	var assignment Assignment

	err = scanAssignment(
		tx.QueryRow(
			ctx,
			`
			INSERT INTO extinguisher_location_assignments (
				equipment_location_id,
				fire_extinguisher_id,
				assignment_reason,
				notes
			)
			VALUES ($1, $2, $3, $4)
			RETURNING
				id,
				equipment_location_id,
				fire_extinguisher_id,
				assigned_at,
				unassigned_at,
				assignment_reason,
				unassignment_reason,
				notes,
				created_by_user_id,
				created_at
			`,
			input.EquipmentLocationID,
			extinguisherID,
			input.AssignmentReason,
			input.Notes,
		),
		&assignment,
	)
	if err != nil {
		return Assignment{}, mapAssignmentDatabaseError(err)
	}

	_, err = tx.Exec(
		ctx,
		`
		UPDATE fire_extinguishers
		SET lifecycle_status = $1,
		    updated_at = now()
		WHERE id = $2
		`,
		extinguishers.LifecycleActiveAtCustomer,
		extinguisherID,
	)
	if err != nil {
		return Assignment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, mapAssignmentDatabaseError(err)
	}

	return assignment, nil
}

func (r *Repository) ListByExtinguisher(
	ctx context.Context,
	extinguisherID uuid.UUID,
) ([]Assignment, error) {
	var extinguisherIDFromDatabase uuid.UUID

	err := r.db.QueryRow(
		ctx,
		`
		SELECT id
		FROM fire_extinguishers
		WHERE id = $1
		  AND archived_at IS NULL
		`,
		extinguisherID,
	).Scan(&extinguisherIDFromDatabase)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrExtinguisherNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			equipment_location_id,
			fire_extinguisher_id,
			assigned_at,
			unassigned_at,
			assignment_reason,
			unassignment_reason,
			notes,
			created_by_user_id,
			created_at
		FROM extinguisher_location_assignments
		WHERE fire_extinguisher_id = $1
		ORDER BY assigned_at DESC, id DESC
		`,
		extinguisherID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := make([]Assignment, 0)

	for rows.Next() {
		var assignment Assignment

		if err := scanAssignment(rows, &assignment); err != nil {
			return nil, err
		}

		assignments = append(assignments, assignment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

func isIssuableLifecycleStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case extinguishers.LifecycleIssuableStock,
		extinguishers.LifecycleRepairedIssuable:
		return true
	default:
		return false
	}
}

func scanAssignment(scanner rowScanner, assignment *Assignment) error {
	return scanner.Scan(
		&assignment.ID,
		&assignment.EquipmentLocationID,
		&assignment.FireExtinguisherID,
		&assignment.AssignedAt,
		&assignment.UnassignedAt,
		&assignment.AssignmentReason,
		&assignment.UnassignmentReason,
		&assignment.Notes,
		&assignment.CreatedByUserID,
		&assignment.CreatedAt,
	)
}

func mapAssignmentDatabaseError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_active_assignment_per_extinguisher":
			return ErrExtinguisherAlreadyAssigned
		case "uq_active_assignment_per_location":
			return ErrEquipmentLocationAlreadyUsed
		}
	}

	return err
}
