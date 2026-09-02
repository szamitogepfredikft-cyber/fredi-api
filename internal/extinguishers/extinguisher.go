package extinguishers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrExtinguisherNotFound = errors.New("extinguisher not found")
var ErrOKFNumberAlreadyExists = errors.New("OKF number already exists")

const (
	LifecycleIssuableStock       = "ISSUABLE_STOCK"
	LifecycleReserved            = "RESERVED"
	LifecycleActiveAtCustomer    = "ACTIVE_AT_CUSTOMER"
	LifecycleReturned            = "RETURNED"
	LifecycleAwaitingMaintenance = "AWAITING_MAINTENANCE"
	LifecycleUnderRepair         = "UNDER_REPAIR"
	LifecycleRepairedIssuable    = "REPAIRED_ISSUABLE"
	LifecycleAwaitingDisposal    = "AWAITING_DISPOSAL"
	LifecycleDisposed            = "DISPOSED"
)

var validLifecycleStatuses = map[string]struct{}{
	LifecycleIssuableStock:       {},
	LifecycleReserved:            {},
	LifecycleActiveAtCustomer:    {},
	LifecycleReturned:            {},
	LifecycleAwaitingMaintenance: {},
	LifecycleUnderRepair:         {},
	LifecycleRepairedIssuable:    {},
	LifecycleAwaitingDisposal:    {},
	LifecycleDisposed:            {},
}

type Extinguisher struct {
	ID                      uuid.UUID  `json:"id"`
	OKFNumber               *string    `json:"okf_number,omitempty"`
	SerialNumber            *string    `json:"serial_number,omitempty"`
	ExtinguisherTypeCode    string     `json:"extinguisher_type_code"`
	ExtinguisherTypeDisplay string     `json:"extinguisher_type_display"`
	ExtinguishingAgent      *string    `json:"extinguishing_agent,omitempty"`
	CapacityKG              *float64   `json:"capacity_kg,omitempty"`
	LifecycleStatus         string     `json:"lifecycle_status"`
	AcquiredAt              *time.Time `json:"acquired_at,omitempty"`
	ManufacturedAt          *time.Time `json:"manufactured_at,omitempty"`
	Notes                   *string    `json:"notes,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type CreateInput struct {
	OKFNumber               *string  `json:"okf_number"`
	SerialNumber            *string  `json:"serial_number"`
	ExtinguisherTypeCode    string   `json:"extinguisher_type_code"`
	ExtinguisherTypeDisplay string   `json:"extinguisher_type_display"`
	ExtinguishingAgent      *string  `json:"extinguishing_agent"`
	CapacityKG              *float64 `json:"capacity_kg"`
	LifecycleStatus         string   `json:"lifecycle_status"`
	AcquiredAt              *string  `json:"acquired_at"`
	ManufacturedAt          *string  `json:"manufactured_at"`
	Notes                   *string  `json:"notes"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (Extinguisher, error) {
	const query = `
		INSERT INTO fire_extinguishers (
			okf_number,
			serial_number,
			extinguisher_type_code,
			extinguisher_type_display,
			extinguishing_agent,
			capacity_kg,
			lifecycle_status,
			acquired_at,
			manufactured_at,
			notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING
			id,
			okf_number,
			serial_number,
			extinguisher_type_code,
			extinguisher_type_display,
			extinguishing_agent,
			capacity_kg,
			lifecycle_status,
			acquired_at,
			manufactured_at,
			notes,
			created_at,
			updated_at
	`

	acquiredAt, err := parseOptionalDate(input.AcquiredAt)
	if err != nil {
		return Extinguisher{}, err
	}

	manufacturedAt, err := parseOptionalDate(input.ManufacturedAt)
	if err != nil {
		return Extinguisher{}, err
	}

	var extinguisher Extinguisher

	err = r.db.QueryRow(
		ctx,
		query,
		optionalTrimmedString(input.OKFNumber),
		optionalTrimmedString(input.SerialNumber),
		strings.TrimSpace(input.ExtinguisherTypeCode),
		strings.TrimSpace(input.ExtinguisherTypeDisplay),
		optionalTrimmedString(input.ExtinguishingAgent),
		input.CapacityKG,
		strings.TrimSpace(input.LifecycleStatus),
		acquiredAt,
		manufacturedAt,
		optionalTrimmedString(input.Notes),
	).Scan(
		&extinguisher.ID,
		&extinguisher.OKFNumber,
		&extinguisher.SerialNumber,
		&extinguisher.ExtinguisherTypeCode,
		&extinguisher.ExtinguisherTypeDisplay,
		&extinguisher.ExtinguishingAgent,
		&extinguisher.CapacityKG,
		&extinguisher.LifecycleStatus,
		&extinguisher.AcquiredAt,
		&extinguisher.ManufacturedAt,
		&extinguisher.Notes,
		&extinguisher.CreatedAt,
		&extinguisher.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Extinguisher{}, ErrOKFNumberAlreadyExists
		}

		return Extinguisher{}, err
	}

	return extinguisher, nil
}

func (r *Repository) List(ctx context.Context) ([]Extinguisher, error) {
	const query = `
		SELECT
			id,
			okf_number,
			serial_number,
			extinguisher_type_code,
			extinguisher_type_display,
			extinguishing_agent,
			capacity_kg,
			lifecycle_status,
			acquired_at,
			manufactured_at,
			notes,
			created_at,
			updated_at
		FROM fire_extinguishers
		WHERE archived_at IS NULL
		ORDER BY lifecycle_status ASC, okf_number ASC NULLS LAST, id ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	extinguisherList := make([]Extinguisher, 0)

	for rows.Next() {
		extinguisher, err := scanExtinguisher(rows)
		if err != nil {
			return nil, err
		}

		extinguisherList = append(extinguisherList, extinguisher)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return extinguisherList, nil
}

func (r *Repository) GetByID(ctx context.Context, extinguisherID uuid.UUID) (Extinguisher, error) {
	const query = `
		SELECT
			id,
			okf_number,
			serial_number,
			extinguisher_type_code,
			extinguisher_type_display,
			extinguishing_agent,
			capacity_kg,
			lifecycle_status,
			acquired_at,
			manufactured_at,
			notes,
			created_at,
			updated_at
		FROM fire_extinguishers
		WHERE id = $1
		  AND archived_at IS NULL
	`

	extinguisher, err := scanExtinguisher(r.db.QueryRow(ctx, query, extinguisherID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Extinguisher{}, ErrExtinguisherNotFound
		}

		return Extinguisher{}, err
	}

	return extinguisher, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanExtinguisher(row rowScanner) (Extinguisher, error) {
	var extinguisher Extinguisher

	err := row.Scan(
		&extinguisher.ID,
		&extinguisher.OKFNumber,
		&extinguisher.SerialNumber,
		&extinguisher.ExtinguisherTypeCode,
		&extinguisher.ExtinguisherTypeDisplay,
		&extinguisher.ExtinguishingAgent,
		&extinguisher.CapacityKG,
		&extinguisher.LifecycleStatus,
		&extinguisher.AcquiredAt,
		&extinguisher.ManufacturedAt,
		&extinguisher.Notes,
		&extinguisher.CreatedAt,
		&extinguisher.UpdatedAt,
	)
	if err != nil {
		return Extinguisher{}, err
	}

	return extinguisher, nil
}

func IsValidLifecycleStatus(value string) bool {
	_, ok := validLifecycleStatuses[strings.TrimSpace(value)]
	return ok
}

func parseOptionalDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, errors.New("invalid date; expected YYYY-MM-DD")
	}

	return &parsed, nil
}

func optionalTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
