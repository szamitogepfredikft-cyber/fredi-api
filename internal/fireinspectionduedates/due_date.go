package fireinspectionduedates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionjobs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DueTypeAnnualInspection  = "ANNUAL_INSPECTION"
	DueTypeMaintenance5Year  = "MAINTENANCE_5_YEAR"
	DueTypeMaintenance10Year = "MAINTENANCE_10_YEAR"
	DueTypeMaintenance15Year = "MAINTENANCE_15_YEAR"
	DueTypeRepairFollowUp    = "REPAIR_FOLLOW_UP"
	DueTypeCustom            = "CUSTOM"

	StatusPlanned        = "TERVEZETT"
	StatusCurrent        = "AKTUALIS"
	StatusContactPending = "KAPCSOLATFELVETELRE_VAR"
	StatusNegotiating    = "EGYEZTETES_ALATT"
	StatusScheduled      = "IDOPONT_EGYEZTETVE"
	StatusJobCreated     = "MUNKALAP_LETREHOZVA"
	StatusCompleted      = "ELVEGEZVE"
	StatusRescheduled    = "ATUTEMEZVE"
	StatusCancelled      = "TOROLVE"
)

var (
	ErrDueDateNotFound        = errors.New("fire inspection due date not found")
	ErrCustomerOrSiteNotFound = errors.New("customer or site not found")
	ErrInvalidDueType         = errors.New("invalid fire inspection due date type")
	ErrInvalidStatus          = errors.New("invalid fire inspection due date status")
	ErrInvalidCoverageYear    = errors.New("invalid coverage year")
	ErrCancelReasonRequired   = errors.New("cancel reason is required")
)

type DueDate struct {
	ID         uuid.UUID `json:"id"`
	CustomerID uuid.UUID `json:"customer_id"`
	SiteID     uuid.UUID `json:"site_id"`

	SourceFireInspectionJobID *uuid.UUID `json:"source_fire_inspection_job_id,omitempty"`
	FireInspectionJobID       *uuid.UUID `json:"fire_inspection_job_id,omitempty"`

	DueType string `json:"due_type"`
	Status  string `json:"status"`

	DueDate      fireinspectionjobs.Date `json:"due_date"`
	CoverageYear *int                    `json:"coverage_year,omitempty"`

	ContactNextAt *fireinspectionjobs.Date `json:"contact_next_at,omitempty"`
	ScheduledFor  *fireinspectionjobs.Date `json:"scheduled_for,omitempty"`

	Notes            *string `json:"notes,omitempty"`
	RescheduleReason *string `json:"reschedule_reason,omitempty"`

	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListItem struct {
	ID uuid.UUID `json:"id"`

	CustomerID   uuid.UUID `json:"customer_id"`
	CustomerName string    `json:"customer_name"`

	SiteID      uuid.UUID `json:"site_id"`
	SiteName    string    `json:"site_name"`
	SiteAddress string    `json:"site_address"`

	DueType      string                  `json:"due_type"`
	Status       string                  `json:"status"`
	DueDate      fireinspectionjobs.Date `json:"due_date"`
	CoverageYear *int                    `json:"coverage_year,omitempty"`

	ContactNextAt *fireinspectionjobs.Date `json:"contact_next_at,omitempty"`
	ScheduledFor  *fireinspectionjobs.Date `json:"scheduled_for,omitempty"`

	SourceFireInspectionJobID *uuid.UUID `json:"source_fire_inspection_job_id,omitempty"`
	FireInspectionJobID       *uuid.UUID `json:"fire_inspection_job_id,omitempty"`

	ContactID                     *uuid.UUID `json:"contact_id,omitempty"`
	ContactName                   *string    `json:"contact_name,omitempty"`
	ContactRoleTitle              *string    `json:"contact_role_title,omitempty"`
	ContactPhone                  *string    `json:"contact_phone,omitempty"`
	ContactEmail                  *string    `json:"contact_email,omitempty"`
	ContactPreferredContactMethod *string    `json:"contact_preferred_contact_method,omitempty"`

	Notes            *string   `json:"notes,omitempty"`
	RescheduleReason *string   `json:"reschedule_reason,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ListResponse struct {
	Items []ListItem `json:"items"`
	Total int        `json:"total"`
}

type ListInput struct {
	Query            string
	Status           string
	DueType          string
	From             *fireinspectionjobs.Date
	To               *fireinspectionjobs.Date
	IncludeCancelled bool
}

type CreateInput struct {
	CustomerID   uuid.UUID                `json:"customer_id"`
	SiteID       uuid.UUID                `json:"site_id"`
	DueType      string                   `json:"due_type"`
	DueDate      *fireinspectionjobs.Date `json:"due_date"`
	CoverageYear *int                     `json:"coverage_year"`
	Notes        *string                  `json:"notes"`
}

type UpdateInput struct {
	Status           *string                  `json:"status"`
	ContactNextAt    *fireinspectionjobs.Date `json:"contact_next_at"`
	ScheduledFor     *fireinspectionjobs.Date `json:"scheduled_for"`
	Notes            *string                  `json:"notes"`
	RescheduleReason *string                  `json:"reschedule_reason"`
}

type SyncResult struct {
	Created  int `json:"created"`
	Existing int `json:"existing"`
	Skipped  int `json:"skipped"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	input CreateInput,
) (DueDate, error) {
	dueType := strings.TrimSpace(input.DueType)
	if !isValidDueType(dueType) {
		return DueDate{}, ErrInvalidDueType
	}

	if input.DueDate == nil || input.DueDate.IsZero() {
		return DueDate{}, errors.New("due_date is required")
	}

	if dueType == DueTypeAnnualInspection {
		if input.CoverageYear == nil || *input.CoverageYear < 2000 || *input.CoverageYear > 2100 {
			return DueDate{}, ErrInvalidCoverageYear
		}
	}

	const query = `
		INSERT INTO fire_inspection_due_dates (
			customer_id, site_id, due_type, status, due_date, coverage_year, notes
		)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE EXISTS (
			SELECT 1 FROM customers
			WHERE id = $1 AND archived_at IS NULL
		)
		AND EXISTS (
			SELECT 1 FROM sites
			WHERE id = $2 AND customer_id = $1 AND archived_at IS NULL
		)
		RETURNING
			id, customer_id, site_id, source_fire_inspection_job_id,
			fire_inspection_job_id, due_type, status, due_date, coverage_year,
			contact_next_at, scheduled_for, notes, reschedule_reason,
			completed_at, cancelled_at, created_at, updated_at
	`

	dueDate, err := scanDueDate(r.db.QueryRow(
		ctx,
		query,
		input.CustomerID,
		input.SiteID,
		dueType,
		StatusPlanned,
		input.DueDate,
		input.CoverageYear,
		normalizedOptionalString(input.Notes),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return DueDate{}, ErrCustomerOrSiteNotFound
	}
	if err != nil {
		return DueDate{}, err
	}

	return dueDate, nil
}

func (r *Repository) SyncAnnual(ctx context.Context) (SyncResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const candidatesQuery = `
		WITH ranked_jobs AS (
			SELECT
				j.id,
				j.customer_id,
				j.site_id,
				COALESCE(j.performed_at::date, j.scheduled_for) AS performed_date,
				ROW_NUMBER() OVER (
					PARTITION BY j.customer_id, j.site_id
					ORDER BY
						COALESCE(j.performed_at::date, j.scheduled_for) DESC,
						j.updated_at DESC,
						j.id DESC
				) AS row_number
			FROM fire_inspection_jobs j
			JOIN customers c ON c.id = j.customer_id AND c.archived_at IS NULL
			JOIN sites s ON s.id = j.site_id AND s.archived_at IS NULL
			WHERE j.status = 'COMPLETED'
			  AND j.archived_at IS NULL
			  AND COALESCE(j.performed_at::date, j.scheduled_for) IS NOT NULL
		)
		SELECT
			id,
			customer_id,
			site_id,
			(performed_date + interval '1 year')::date AS due_date,
			EXTRACT(YEAR FROM (performed_date + interval '1 year'))::int AS coverage_year
		FROM ranked_jobs
		WHERE row_number = 1
		ORDER BY due_date, customer_id, site_id
	`

	type candidate struct {
		sourceJobID  uuid.UUID
		customerID   uuid.UUID
		siteID       uuid.UUID
		dueDate      time.Time
		coverageYear int
	}

	rows, err := tx.Query(ctx, candidatesQuery)
	if err != nil {
		return SyncResult{}, err
	}

	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate

		if err := rows.Scan(
			&item.sourceJobID,
			&item.customerID,
			&item.siteID,
			&item.dueDate,
			&item.coverageYear,
		); err != nil {
			rows.Close()
			return SyncResult{}, err
		}

		candidates = append(candidates, item)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return SyncResult{}, err
	}
	rows.Close()

	result := SyncResult{}

	for _, item := range candidates {
		var existingStatus string
		err := tx.QueryRow(
			ctx,
			`
			SELECT status
			FROM fire_inspection_due_dates
			WHERE customer_id = $1
			  AND site_id = $2
			  AND due_type = $3
			  AND coverage_year = $4
			  AND archived_at IS NULL
			ORDER BY created_at DESC
			LIMIT 1
			`,
			item.customerID,
			item.siteID,
			DueTypeAnnualInspection,
			item.coverageYear,
		).Scan(&existingStatus)

		if err == nil {
			result.Existing++
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return SyncResult{}, err
		}

		_, err = tx.Exec(
			ctx,
			`
			INSERT INTO fire_inspection_due_dates (
				customer_id,
				site_id,
				source_fire_inspection_job_id,
				due_type,
				status,
				due_date,
				coverage_year,
				notes
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
			item.customerID,
			item.siteID,
			item.sourceJobID,
			DueTypeAnnualInspection,
			StatusPlanned,
			item.dueDate,
			item.coverageYear,
			"Automatikusan létrehozva a legutóbbi lezárt munkalap alapján.",
		)
		if err != nil {
			return SyncResult{}, err
		}

		result.Created++
	}

	if err := tx.Commit(ctx); err != nil {
		return SyncResult{}, err
	}

	return result, nil
}

func (r *Repository) Update(
	ctx context.Context,
	dueDateID uuid.UUID,
	input UpdateInput,
) (DueDate, error) {
	if input.Status == nil &&
		input.ContactNextAt == nil &&
		input.ScheduledFor == nil &&
		input.Notes == nil &&
		input.RescheduleReason == nil {
		return DueDate{}, errors.New("at least one field is required")
	}

	var currentStatus string
	err := r.db.QueryRow(
		ctx,
		`
		SELECT status
		FROM fire_inspection_due_dates
		WHERE id = $1
		  AND archived_at IS NULL
		FOR UPDATE
		`,
		dueDateID,
	).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return DueDate{}, ErrDueDateNotFound
	}
	if err != nil {
		return DueDate{}, err
	}

	status := currentStatus
	if input.Status != nil {
		status = strings.TrimSpace(*input.Status)
		if !isValidStatus(status) {
			return DueDate{}, ErrInvalidStatus
		}
	}

	rescheduleReason := normalizedOptionalString(input.RescheduleReason)
	if status == StatusCancelled && rescheduleReason == nil {
		return DueDate{}, ErrCancelReasonRequired
	}

	const query = `
		UPDATE fire_inspection_due_dates
		SET
			status = $2,
			contact_next_at = COALESCE($3, contact_next_at),
			scheduled_for = COALESCE($4, scheduled_for),
			notes = COALESCE($5, notes),
			reschedule_reason = CASE
				WHEN $6::text IS NULL THEN reschedule_reason
				ELSE $6
			END,
			cancelled_at = CASE
				WHEN $2 = 'TOROLVE' THEN COALESCE(cancelled_at, NOW())
				WHEN status = 'TOROLVE' THEN NULL
				ELSE cancelled_at
			END,
			completed_at = CASE
				WHEN $2 = 'ELVEGEZVE' THEN COALESCE(completed_at, NOW())
				WHEN status = 'ELVEGEZVE' THEN NULL
				ELSE completed_at
			END,
			updated_at = NOW()
		WHERE id = $1
		  AND archived_at IS NULL
		RETURNING
			id, customer_id, site_id, source_fire_inspection_job_id,
			fire_inspection_job_id, due_type, status, due_date, coverage_year,
			contact_next_at, scheduled_for, notes, reschedule_reason,
			completed_at, cancelled_at, created_at, updated_at
	`

	dueDate, err := scanDueDate(r.db.QueryRow(
		ctx,
		query,
		dueDateID,
		status,
		input.ContactNextAt,
		input.ScheduledFor,
		normalizedOptionalString(input.Notes),
		rescheduleReason,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return DueDate{}, ErrDueDateNotFound
	}
	if err != nil {
		return DueDate{}, err
	}

	return dueDate, nil
}

func (r *Repository) List(
	ctx context.Context,
	input ListInput,
) ([]ListItem, error) {
	queryParts := []string{
		`
		SELECT
			d.id,
			c.id,
			c.name,
			s.id,
			s.name,
			s.address_display,
			d.due_type,
			d.status,
			d.due_date,
			d.coverage_year,
			d.contact_next_at,
			d.scheduled_for,
			d.source_fire_inspection_job_id,
			d.fire_inspection_job_id,
			COALESCE(site_contact.id, customer_contact.id) AS contact_id,
			COALESCE(site_contact.name, customer_contact.name) AS contact_name,
			COALESCE(site_contact.role_title, customer_contact.role_title) AS contact_role_title,
			COALESCE(site_contact.phone, customer_contact.phone) AS contact_phone,
			COALESCE(site_contact.email, customer_contact.email) AS contact_email,
			COALESCE(
				site_contact.preferred_contact_method,
				customer_contact.preferred_contact_method
			) AS contact_preferred_contact_method,
			d.notes,
			d.reschedule_reason,
			d.updated_at
		FROM fire_inspection_due_dates d
		JOIN customers c
			ON c.id = d.customer_id
		JOIN sites s
			ON s.id = d.site_id
		LEFT JOIN customer_contacts site_contact
			ON site_contact.site_id = d.site_id
		   AND site_contact.customer_id = d.customer_id
		   AND site_contact.is_primary = TRUE
		   AND site_contact.archived_at IS NULL
		LEFT JOIN customer_contacts customer_contact
			ON customer_contact.customer_id = d.customer_id
		   AND customer_contact.site_id IS NULL
		   AND customer_contact.is_primary = TRUE
		   AND customer_contact.archived_at IS NULL
		WHERE d.archived_at IS NULL
		  AND c.archived_at IS NULL
		  AND s.archived_at IS NULL
		`,
	}

	args := make([]any, 0, 6)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if !input.IncludeCancelled {
		queryParts = append(queryParts, "AND d.status <> 'TOROLVE'")
	}

	if query := strings.TrimSpace(input.Query); query != "" {
		placeholder := addArg("%" + query + "%")
		queryParts = append(queryParts, `
			AND (
				c.name ILIKE `+placeholder+`
				OR s.name ILIKE `+placeholder+`
				OR s.address_display ILIKE `+placeholder+`
			)
		`)
	}

	if status := strings.TrimSpace(input.Status); status != "" {
		if !isValidStatus(status) {
			return nil, ErrInvalidStatus
		}
		placeholder := addArg(status)
		queryParts = append(queryParts, "AND d.status = "+placeholder)
	}

	if dueType := strings.TrimSpace(input.DueType); dueType != "" {
		if !isValidDueType(dueType) {
			return nil, ErrInvalidDueType
		}
		placeholder := addArg(dueType)
		queryParts = append(queryParts, "AND d.due_type = "+placeholder)
	}

	if input.From != nil && !input.From.IsZero() {
		placeholder := addArg(input.From)
		queryParts = append(queryParts, "AND d.due_date >= "+placeholder)
	}

	if input.To != nil && !input.To.IsZero() {
		placeholder := addArg(input.To)
		queryParts = append(queryParts, "AND d.due_date <= "+placeholder)
	}

	queryParts = append(queryParts, `
		ORDER BY d.due_date ASC, c.name ASC, s.name ASC
	`)

	rows, err := r.db.Query(ctx, strings.Join(queryParts, "\n"), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ListItem, 0)

	for rows.Next() {
		var item ListItem

		if err := rows.Scan(
			&item.ID,
			&item.CustomerID,
			&item.CustomerName,
			&item.SiteID,
			&item.SiteName,
			&item.SiteAddress,
			&item.DueType,
			&item.Status,
			&item.DueDate,
			&item.CoverageYear,
			&item.ContactNextAt,
			&item.ScheduledFor,
			&item.SourceFireInspectionJobID,
			&item.FireInspectionJobID,
			&item.ContactID,
			&item.ContactName,
			&item.ContactRoleTitle,
			&item.ContactPhone,
			&item.ContactEmail,
			&item.ContactPreferredContactMethod,
			&item.Notes,
			&item.RescheduleReason,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDueDate(row rowScanner) (DueDate, error) {
	var dueDate DueDate

	err := row.Scan(
		&dueDate.ID,
		&dueDate.CustomerID,
		&dueDate.SiteID,
		&dueDate.SourceFireInspectionJobID,
		&dueDate.FireInspectionJobID,
		&dueDate.DueType,
		&dueDate.Status,
		&dueDate.DueDate,
		&dueDate.CoverageYear,
		&dueDate.ContactNextAt,
		&dueDate.ScheduledFor,
		&dueDate.Notes,
		&dueDate.RescheduleReason,
		&dueDate.CompletedAt,
		&dueDate.CancelledAt,
		&dueDate.CreatedAt,
		&dueDate.UpdatedAt,
	)
	if err != nil {
		return DueDate{}, err
	}

	return dueDate, nil
}

func isValidDueType(value string) bool {
	switch value {
	case
		DueTypeAnnualInspection,
		DueTypeMaintenance5Year,
		DueTypeMaintenance10Year,
		DueTypeMaintenance15Year,
		DueTypeRepairFollowUp,
		DueTypeCustom:
		return true
	default:
		return false
	}
}

func isValidStatus(value string) bool {
	switch value {
	case
		StatusPlanned,
		StatusCurrent,
		StatusContactPending,
		StatusNegotiating,
		StatusScheduled,
		StatusJobCreated,
		StatusCompleted,
		StatusRescheduled,
		StatusCancelled:
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

func DecodeCreateInput(data []byte) (CreateInput, error) {
	var input CreateInput

	if err := json.Unmarshal(data, &input); err != nil {
		return CreateInput{}, err
	}

	return input, nil
}
