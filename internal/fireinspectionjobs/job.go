package fireinspectionjobs

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCustomerOrSiteNotFound = errors.New("customer or site not found")
var ErrJobNotFound = errors.New("fire inspection job not found")

type Date struct {
	time.Time
}

func (d *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("date must be a JSON string in YYYY-MM-DD format")
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return fmt.Errorf("invalid date %q: expected YYYY-MM-DD", value)
	}

	d.Time = parsed

	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format("2006-01-02"))
}

func (d *Date) Scan(src any) error {
	if src == nil {
		d.Time = time.Time{}
		return nil
	}

	value, ok := src.(time.Time)
	if !ok {
		return fmt.Errorf("cannot scan %T into Date", src)
	}

	d.Time = value

	return nil
}

func (d Date) Value() (driver.Value, error) {
	return d.Time, nil
}

type Job struct {
	ID         uuid.UUID `json:"id"`
	CustomerID uuid.UUID `json:"customer_id"`
	SiteID     uuid.UUID `json:"site_id"`
	Status     string    `json:"status"`

	ScheduledFor      *Date      `json:"scheduled_for,omitempty"`
	PerformedAt       *time.Time `json:"performed_at,omitempty"`
	InspectionYear    *int       `json:"inspection_year,omitempty"`
	InspectionQuarter *string    `json:"inspection_quarter,omitempty"`

	IssuedAt       *time.Time `json:"issued_at,omitempty"`
	IssuedByUserID *uuid.UUID `json:"issued_by_user_id,omitempty"`

	IssuedByNameSnapshot    *string `json:"issued_by_name_snapshot,omitempty"`
	IssuedByCompanySnapshot *string `json:"issued_by_company_snapshot,omitempty"`
	IssuedByPhoneSnapshot   *string `json:"issued_by_phone_snapshot,omitempty"`
	IssuedByEmailSnapshot   *string `json:"issued_by_email_snapshot,omitempty"`

	InspectorNameSnapshot        *string `json:"inspector_name_snapshot,omitempty"`
	InspectorCertificateSnapshot *string `json:"inspector_certificate_snapshot,omitempty"`
	RepairerNameSnapshot         *string `json:"repairer_name_snapshot,omitempty"`

	Notes           *string    `json:"notes,omitempty"`
	CreatedByUserID *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CreateInput struct {
	CustomerID   uuid.UUID `json:"customer_id"`
	SiteID       uuid.UUID `json:"site_id"`
	ScheduledFor *Date     `json:"scheduled_for"`
	Notes        *string   `json:"notes"`

	IssuedByName    *string `json:"issued_by_name"`
	IssuedByCompany *string `json:"issued_by_company"`
	IssuedByPhone   *string `json:"issued_by_phone"`
	IssuedByEmail   *string `json:"issued_by_email"`

	InspectorName        *string `json:"inspector_name"`
	InspectorCertificate *string `json:"inspector_certificate"`
	RepairerName         *string `json:"repairer_name"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (Job, error) {
	inspectionQuarter, inspectionYear := quarterFromDate(input.ScheduledFor)

	const createQuery = `
		INSERT INTO fire_inspection_jobs (
			customer_id,
			site_id,
			scheduled_for,
			inspection_year,
			inspection_quarter,
			issued_at,
			issued_by_name_snapshot,
			issued_by_company_snapshot,
			issued_by_phone_snapshot,
			issued_by_email_snapshot,
			inspector_name_snapshot,
			inspector_certificate_snapshot,
			repairer_name_snapshot,
			notes
		)
		SELECT
			$1,
			$2,
			$3,
			$4,
			$5,
			NOW(),
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13
		WHERE EXISTS (
			SELECT 1
			FROM customers
			WHERE id = $1
			  AND archived_at IS NULL
		)
		AND EXISTS (
			SELECT 1
			FROM sites
			WHERE id = $2
			  AND customer_id = $1
			  AND archived_at IS NULL
		)
		RETURNING
			id,
			customer_id,
			site_id,
			status,
			scheduled_for,
			performed_at,
			inspection_year,
			inspection_quarter,
			issued_at,
			issued_by_user_id,
			issued_by_name_snapshot,
			issued_by_company_snapshot,
			issued_by_phone_snapshot,
			issued_by_email_snapshot,
			inspector_name_snapshot,
			inspector_certificate_snapshot,
			repairer_name_snapshot,
			notes,
			created_by_user_id,
			created_at,
			updated_at
	`

	job, err := scanJob(r.db.QueryRow(
		ctx,
		createQuery,
		input.CustomerID,
		input.SiteID,
		input.ScheduledFor,
		inspectionYear,
		inspectionQuarter,
		optionalTrimmedString(input.IssuedByName),
		optionalTrimmedString(input.IssuedByCompany),
		optionalTrimmedString(input.IssuedByPhone),
		optionalTrimmedString(input.IssuedByEmail),
		optionalTrimmedString(input.InspectorName),
		optionalTrimmedString(input.InspectorCertificate),
		optionalTrimmedString(input.RepairerName),
		optionalTrimmedString(input.Notes),
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrCustomerOrSiteNotFound
		}

		return Job{}, err
	}

	return job, nil
}

func (r *Repository) GetByID(ctx context.Context, jobID uuid.UUID) (Job, error) {
	const query = `
		SELECT
			id,
			customer_id,
			site_id,
			status,
			scheduled_for,
			performed_at,
			inspection_year,
			inspection_quarter,
			issued_at,
			issued_by_user_id,
			issued_by_name_snapshot,
			issued_by_company_snapshot,
			issued_by_phone_snapshot,
			issued_by_email_snapshot,
			inspector_name_snapshot,
			inspector_certificate_snapshot,
			repairer_name_snapshot,
			notes,
			created_by_user_id,
			created_at,
			updated_at
		FROM fire_inspection_jobs
		WHERE id = $1
		  AND archived_at IS NULL
	`

	job, err := scanJob(r.db.QueryRow(ctx, query, jobID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrJobNotFound
		}

		return Job{}, err
	}

	return job, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job

	err := row.Scan(
		&job.ID,
		&job.CustomerID,
		&job.SiteID,
		&job.Status,
		&job.ScheduledFor,
		&job.PerformedAt,
		&job.InspectionYear,
		&job.InspectionQuarter,
		&job.IssuedAt,
		&job.IssuedByUserID,
		&job.IssuedByNameSnapshot,
		&job.IssuedByCompanySnapshot,
		&job.IssuedByPhoneSnapshot,
		&job.IssuedByEmailSnapshot,
		&job.InspectorNameSnapshot,
		&job.InspectorCertificateSnapshot,
		&job.RepairerNameSnapshot,
		&job.Notes,
		&job.CreatedByUserID,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func quarterFromDate(value *Date) (*string, *int) {
	if value == nil || value.IsZero() {
		return nil, nil
	}

	year := value.Year()

	var quarter string

	switch value.Month() {
	case time.January, time.February, time.March:
		quarter = "Q1"
	case time.April, time.May, time.June:
		quarter = "Q2"
	case time.July, time.August, time.September:
		quarter = "Q3"
	default:
		quarter = "Q4"
	}

	return &quarter, &year
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
