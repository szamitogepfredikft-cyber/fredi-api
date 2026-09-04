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

var (
	ErrCustomerOrSiteNotFound = errors.New("customer or site not found")
	ErrInspectorNotSelectable = errors.New("inspector is not selectable")
	ErrJobNotFound            = errors.New("fire inspection job not found")
	ErrJobNotEditable         = errors.New("fire inspection job is not editable")
	ErrJobNotCompletable      = errors.New("fire inspection job is not completable")
	ErrUncheckedRows          = errors.New("fire inspection job has unchecked rows")
	ErrJobNotReopenable       = errors.New("fire inspection job is not reopenable")
)

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
	ID           uuid.UUID `json:"id"`
	CustomerID   uuid.UUID `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	SiteAddress  string    `json:"site_address"`
	SiteID       uuid.UUID `json:"site_id"`
	Status       string    `json:"status"`

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
	InspectorPhoneSnapshot       *string `json:"inspector_phone_snapshot,omitempty"`
	InspectorEmailSnapshot       *string `json:"inspector_email_snapshot,omitempty"`
	InspectorCertificateSnapshot *string `json:"inspector_certificate_snapshot,omitempty"`
	RepairerNameSnapshot         *string `json:"repairer_name_snapshot,omitempty"`

	Notes           *string    `json:"notes,omitempty"`
	CreatedByUserID *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ListInput struct {
	View   string
	Status string
	Query  string
	From   *Date
	To     *Date
}

type ListItem struct {
	ID           uuid.UUID `json:"id"`
	CustomerName string    `json:"customer_name"`
	SiteAddress  string    `json:"site_address"`
	SiteName     string    `json:"site_name"`
	Status       string    `json:"status"`

	ScheduledFor *Date `json:"scheduled_for,omitempty"`

	TotalRows     int `json:"total_rows"`
	ProcessedRows int `json:"processed_rows"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListResponse struct {
	Items []ListItem `json:"items"`
	Total int        `json:"total"`
}

type CreateInput struct {
	CustomerID   uuid.UUID `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	SiteAddress  string    `json:"site_address"`
	SiteID       uuid.UUID `json:"site_id"`
	ScheduledFor *Date     `json:"scheduled_for"`
	Notes        *string   `json:"notes"`

	IssuedByName    *string `json:"issued_by_name"`
	IssuedByCompany *string `json:"issued_by_company"`
	IssuedByPhone   *string `json:"issued_by_phone"`
	IssuedByEmail   *string `json:"issued_by_email"`

	InspectorID          *uuid.UUID `json:"inspector_id"`
	InspectorName        *string    `json:"inspector_name"`
	InspectorCertificate *string    `json:"inspector_certificate"`
	RepairerName         *string    `json:"repairer_name"`
}
type optionalString struct {
	Set   bool
	Value *string
}

type optionalDate struct {
	Set   bool
	Value *Date
}

type optionalTime struct {
	Set   bool
	Value *time.Time
}

type UpdateInput struct {
	ScheduledFor         optionalDate
	PerformedAt          optionalTime
	IssuedByName         optionalString
	IssuedByCompany      optionalString
	IssuedByPhone        optionalString
	IssuedByEmail        optionalString
	InspectorName        optionalString
	InspectorCertificate optionalString
	RepairerName         optionalString
	Notes                optionalString
}

func (i *UpdateInput) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	allowedFields := map[string]struct{}{
		"scheduled_for":         {},
		"performed_at":          {},
		"issued_by_name":        {},
		"issued_by_company":     {},
		"issued_by_phone":       {},
		"issued_by_email":       {},
		"inspector_name":        {},
		"inspector_certificate": {},
		"repairer_name":         {},
		"notes":                 {},
	}

	for field := range raw {
		if _, ok := allowedFields[field]; !ok {
			return errors.New("unknown JSON field")
		}
	}

	if err := decodeOptionalDate(raw, "scheduled_for", &i.ScheduledFor); err != nil {
		return err
	}

	if err := decodeOptionalTime(raw, "performed_at", &i.PerformedAt); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "issued_by_name", &i.IssuedByName); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "issued_by_company", &i.IssuedByCompany); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "issued_by_phone", &i.IssuedByPhone); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "issued_by_email", &i.IssuedByEmail); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "inspector_name", &i.InspectorName); err != nil {
		return err
	}

	if err := decodeOptionalString(
		raw,
		"inspector_certificate",
		&i.InspectorCertificate,
	); err != nil {
		return err
	}

	if err := decodeOptionalString(raw, "repairer_name", &i.RepairerName); err != nil {
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

	if string(value) == "null" {
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

func decodeOptionalDate(
	raw map[string]json.RawMessage,
	field string,
	target *optionalDate,
) error {
	value, ok := raw[field]
	if !ok {
		return nil
	}

	target.Set = true

	if string(value) == "null" {
		target.Value = nil
		return nil
	}

	var decoded Date

	if err := json.Unmarshal(value, &decoded); err != nil {
		return err
	}

	target.Value = &decoded

	return nil
}

func decodeOptionalTime(
	raw map[string]json.RawMessage,
	field string,
	target *optionalTime,
) error {
	value, ok := raw[field]
	if !ok {
		return nil
	}

	target.Set = true

	if string(value) == "null" {
		target.Value = nil
		return nil
	}

	var valueString string

	if err := json.Unmarshal(value, &valueString); err != nil {
		return errors.New(field + " must be an RFC3339 timestamp or null")
	}

	parsed, err := time.Parse(time.RFC3339, valueString)
	if err != nil {
		return errors.New(field + " must be an RFC3339 timestamp or null")
	}

	target.Value = &parsed

	return nil
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (Job, error) {
	inspectionQuarter, inspectionYear := quarterFromDate(input.ScheduledFor)

	if input.InspectorID != nil {
		const selectableInspectorQuery = `
			SELECT EXISTS (
				SELECT 1
				FROM inspectors i
				WHERE i.id = $1
				  AND i.archived_at IS NULL
				  AND (
					SELECT COUNT(*)
					FROM inspector_certificates c
					WHERE c.inspector_id = i.id
					  AND c.archived_at IS NULL
				  ) = 1
			)
		`

		var selectable bool

		if err := r.db.QueryRow(
			ctx,
			selectableInspectorQuery,
			*input.InspectorID,
		).Scan(&selectable); err != nil {
			return Job{}, err
		}

		if !selectable {
			return Job{}, ErrInspectorNotSelectable
		}
	}

	const createQuery = `
		WITH selected_inspector AS (
			SELECT
				i.name,
				i.phone,
				i.email,
				c.certificate_number
			FROM inspectors i
			JOIN inspector_certificates c
				ON c.inspector_id = i.id
				AND c.archived_at IS NULL
			WHERE i.id = $10
				AND i.archived_at IS NULL
			AND (
				SELECT COUNT(*)
				FROM inspector_certificates active_certificates
				WHERE active_certificates.inspector_id = i.id
				  AND active_certificates.archived_at IS NULL
			) = 1
		)
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
			inspector_phone_snapshot,
			inspector_email_snapshot,
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
			CASE
				WHEN $10::uuid IS NULL THEN $11
				ELSE (SELECT name FROM selected_inspector)
			END,
			CASE
				WHEN $10::uuid IS NULL THEN NULL
				ELSE (SELECT phone FROM selected_inspector)
			END,
			CASE
				WHEN $10::uuid IS NULL THEN NULL
				ELSE (SELECT email FROM selected_inspector)
			END,
			CASE
				WHEN $10::uuid IS NULL THEN $12
				ELSE (SELECT certificate_number FROM selected_inspector)
			END,
			$13,
			$14
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
		AND (
			$10::uuid IS NULL
			OR EXISTS (SELECT 1 FROM selected_inspector)
		)
		RETURNING id
	`

	var jobID uuid.UUID

	err := r.db.QueryRow(
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
		input.InspectorID,
		optionalTrimmedString(input.InspectorName),
		optionalTrimmedString(input.InspectorCertificate),
		optionalTrimmedString(input.RepairerName),
		optionalTrimmedString(input.Notes),
	).Scan(&jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrCustomerOrSiteNotFound
		}

		return Job{}, err
	}

	return r.GetByID(ctx, jobID)
}

func (r *Repository) Update(
	ctx context.Context,
	jobID uuid.UUID,
	input UpdateInput,
) (Job, error) {
	var status string

	err := r.db.QueryRow(
		ctx,
		`
		SELECT status
		FROM fire_inspection_jobs
		WHERE id = $1
		  AND archived_at IS NULL
		`,
		jobID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}

	if status == "COMPLETED" || status == "CANCELLED" {
		return Job{}, ErrJobNotEditable
	}

	var inspectionQuarter *string
	var inspectionYear *int

	if input.ScheduledFor.Set {
		inspectionQuarter, inspectionYear = quarterFromDate(input.ScheduledFor.Value)
	}

	const query = `
		UPDATE fire_inspection_jobs
		SET
			scheduled_for = CASE
				WHEN $2 THEN $3
				ELSE scheduled_for
			END,
			inspection_year = CASE
				WHEN $2 THEN $4
				ELSE inspection_year
			END,
			inspection_quarter = CASE
				WHEN $2 THEN $5
				ELSE inspection_quarter
			END,
			performed_at = CASE
				WHEN $6 THEN $7
				ELSE performed_at
			END,
			issued_by_name_snapshot = CASE
				WHEN $8 THEN $9
				ELSE issued_by_name_snapshot
			END,
			issued_by_company_snapshot = CASE
				WHEN $10 THEN $11
				ELSE issued_by_company_snapshot
			END,
			issued_by_phone_snapshot = CASE
				WHEN $12 THEN $13
				ELSE issued_by_phone_snapshot
			END,
			issued_by_email_snapshot = CASE
				WHEN $14 THEN $15
				ELSE issued_by_email_snapshot
			END,
			inspector_name_snapshot = CASE
				WHEN $16 THEN $17
				ELSE inspector_name_snapshot
			END,
			inspector_certificate_snapshot = CASE
				WHEN $18 THEN $19
				ELSE inspector_certificate_snapshot
			END,
			repairer_name_snapshot = CASE
				WHEN $20 THEN $21
				ELSE repairer_name_snapshot
			END,
			notes = CASE
				WHEN $22 THEN $23
				ELSE notes
			END,
			updated_at = now()
		WHERE id = $1
		  AND archived_at IS NULL
	`

	_, err = r.db.Exec(
		ctx,
		query,
		jobID,
		input.ScheduledFor.Set,
		input.ScheduledFor.Value,
		inspectionYear,
		inspectionQuarter,
		input.PerformedAt.Set,
		input.PerformedAt.Value,
		input.IssuedByName.Set,
		optionalTrimmedString(input.IssuedByName.Value),
		input.IssuedByCompany.Set,
		optionalTrimmedString(input.IssuedByCompany.Value),
		input.IssuedByPhone.Set,
		optionalTrimmedString(input.IssuedByPhone.Value),
		input.IssuedByEmail.Set,
		optionalTrimmedString(input.IssuedByEmail.Value),
		input.InspectorName.Set,
		optionalTrimmedString(input.InspectorName.Value),
		input.InspectorCertificate.Set,
		optionalTrimmedString(input.InspectorCertificate.Value),
		input.RepairerName.Set,
		optionalTrimmedString(input.RepairerName.Value),
		input.Notes.Set,
		optionalTrimmedString(input.Notes.Value),
	)
	if err != nil {
		return Job{}, err
	}

	return r.GetByID(ctx, jobID)
}

func (r *Repository) List(
	ctx context.Context,
	input ListInput,
) ([]ListItem, error) {
	queryParts := []string{
		`
        SELECT
            j.id,
            c.name AS customer_name,
            s.name AS site_name,
            s.address_display AS site_address,
            j.status,
            j.scheduled_for,
            COUNT(row.id)::int AS total_rows,
            COUNT(row.id) FILTER (
                WHERE row.row_result <> 'NEM_ELLENORIZVE'
            )::int AS processed_rows,
            j.created_at,
            j.updated_at
        FROM fire_inspection_jobs j
        JOIN customers c
            ON c.id = j.customer_id
        JOIN sites s
            ON s.id = j.site_id
        LEFT JOIN fire_inspection_rows row
            ON row.fire_inspection_job_id = j.id
        `,
		`
        WHERE c.archived_at IS NULL
          AND s.archived_at IS NULL
        `,
	}

	args := make([]any, 0, 5)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	view := strings.ToLower(strings.TrimSpace(input.View))

	switch view {
	case "", "open":
		queryParts = append(
			queryParts,
			"AND j.archived_at IS NULL",
			"AND j.status NOT IN ('COMPLETED', 'CANCELLED')",
		)
	case "closed":
		queryParts = append(
			queryParts,
			"AND j.archived_at IS NULL",
			"AND j.status IN ('COMPLETED', 'CANCELLED')",
		)
	case "archived":
		queryParts = append(
			queryParts,
			"AND j.archived_at IS NOT NULL",
		)
	default:
		return nil, errors.New("invalid fire inspection job list view")
	}

	if status := strings.TrimSpace(input.Status); status != "" {
		placeholder := addArg(status)
		queryParts = append(queryParts, "AND j.status = "+placeholder)
	}

	if searchQuery := strings.TrimSpace(input.Query); searchQuery != "" {
		placeholder := addArg("%" + searchQuery + "%")
		queryParts = append(
			queryParts,
			`
            AND (
                c.name ILIKE `+placeholder+`
                OR s.name ILIKE `+placeholder+`
                OR s.address_display ILIKE `+placeholder+`
            )
            `,
		)
	}

	if input.From != nil && !input.From.IsZero() {
		placeholder := addArg(input.From)
		queryParts = append(queryParts, "AND j.scheduled_for >= "+placeholder)
	}

	if input.To != nil && !input.To.IsZero() {
		placeholder := addArg(input.To)
		queryParts = append(queryParts, "AND j.scheduled_for <= "+placeholder)
	}

	queryParts = append(
		queryParts,
		`
        GROUP BY
            j.id,
            c.name,
            s.name,
            s.address_display
        ORDER BY
            j.scheduled_for ASC NULLS LAST,
            j.updated_at DESC
        `,
	)

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
			&item.CustomerName,
			&item.SiteName,
			&item.SiteAddress,
			&item.Status,
			&item.ScheduledFor,
			&item.TotalRows,
			&item.ProcessedRows,
			&item.CreatedAt,
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

func (r *Repository) Complete(
	ctx context.Context,
	jobID uuid.UUID,
) (Job, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var status string

	err = tx.QueryRow(
		ctx,
		`
                SELECT status
                FROM fire_inspection_jobs
                WHERE id = $1
                  AND archived_at IS NULL
                FOR UPDATE
                `,
		jobID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}

	if status == "COMPLETED" || status == "CANCELLED" {
		return Job{}, ErrJobNotCompletable
	}

	var uncheckedRows int

	err = tx.QueryRow(
		ctx,
		`
                SELECT COUNT(*)::int
                FROM fire_inspection_rows
                WHERE fire_inspection_job_id = $1
                  AND row_result = 'NEM_ELLENORIZVE'
                `,
		jobID,
	).Scan(&uncheckedRows)
	if err != nil {
		return Job{}, err
	}

	if uncheckedRows > 0 {
		return Job{}, ErrUncheckedRows
	}

	_, err = tx.Exec(
		ctx,
		`
                UPDATE fire_inspection_jobs
                SET
                        status = 'COMPLETED',
                        performed_at = COALESCE(performed_at, NOW()),
                        updated_at = NOW()
                WHERE id = $1
                  AND archived_at IS NULL
                `,
		jobID,
	)
	if err != nil {
		return Job{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}

	return r.GetByID(ctx, jobID)
}

func (r *Repository) Reopen(
	ctx context.Context,
	jobID uuid.UUID,
) (Job, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var status string

	err = tx.QueryRow(
		ctx,
		`
                SELECT status
                FROM fire_inspection_jobs
                WHERE id = $1
                  AND archived_at IS NULL
                FOR UPDATE
                `,
		jobID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}

	if status != "COMPLETED" {
		return Job{}, ErrJobNotReopenable
	}

	_, err = tx.Exec(
		ctx,
		`
                UPDATE fire_inspection_jobs
                SET
                        status = 'IN_PROGRESS',
                        updated_at = NOW()
                WHERE id = $1
                  AND archived_at IS NULL
                `,
		jobID,
	)
	if err != nil {
		return Job{}, err
	}

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
                        inspector_phone_snapshot,
                        inspector_email_snapshot,
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

	job, err := scanJob(tx.QueryRow(ctx, query, jobID))
	if err != nil {
		return Job{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}

	return job, nil
}

func (r *Repository) GetByID(ctx context.Context, jobID uuid.UUID) (Job, error) {
	const query = `
		SELECT
			j.id,
			j.customer_id,
			c.name AS customer_name,
                        s.address_display AS site_address,
			j.site_id,
			j.status,
			j.scheduled_for,
			j.performed_at,
			j.inspection_year,
			j.inspection_quarter,
			j.issued_at,
			j.issued_by_user_id,
			j.issued_by_name_snapshot,
			j.issued_by_company_snapshot,
			j.issued_by_phone_snapshot,
			j.issued_by_email_snapshot,
			j.inspector_name_snapshot,
			j.inspector_phone_snapshot,
			j.inspector_email_snapshot,
			j.inspector_certificate_snapshot,
			j.repairer_name_snapshot,
			j.notes,
			j.created_by_user_id,
			j.created_at,
			j.updated_at
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
		&job.CustomerName,
		&job.SiteAddress,
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
		&job.InspectorPhoneSnapshot,
		&job.InspectorEmailSnapshot,
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
