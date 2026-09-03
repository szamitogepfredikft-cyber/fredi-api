package customercontacts

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

var (
	ErrCustomerNotFound     = errors.New("customer not found")
	ErrSiteNotFound         = errors.New("site not found")
	ErrSiteDoesNotBelong    = errors.New("site does not belong to customer")
	ErrContactNotFound      = errors.New("customer contact not found")
	ErrPrimaryContactExists = errors.New("primary customer contact already exists")
	ErrInvalidContactMethod = errors.New("invalid preferred contact method")
)

const (
	ContactMethodPhone    = "PHONE"
	ContactMethodEmail    = "EMAIL"
	ContactMethodPersonal = "PERSONAL"
	ContactMethodOther    = "OTHER"
)

type Contact struct {
	ID         uuid.UUID  `json:"id"`
	CustomerID uuid.UUID  `json:"customer_id"`
	SiteID     *uuid.UUID `json:"site_id,omitempty"`

	Name      string  `json:"name"`
	RoleTitle *string `json:"role_title,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Email     *string `json:"email,omitempty"`

	PreferredContactMethod *string `json:"preferred_contact_method,omitempty"`
	IsPrimary              bool    `json:"is_primary"`
	Notes                  *string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateInput struct {
	SiteID *uuid.UUID `json:"site_id"`

	Name      string  `json:"name"`
	RoleTitle *string `json:"role_title"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`

	PreferredContactMethod *string `json:"preferred_contact_method"`
	IsPrimary              bool    `json:"is_primary"`
	Notes                  *string `json:"notes"`
}

type UpdateInput struct {
	SiteID *uuid.UUID `json:"site_id"`

	Name      string  `json:"name"`
	RoleTitle *string `json:"role_title"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`

	PreferredContactMethod *string `json:"preferred_contact_method"`
	IsPrimary              bool    `json:"is_primary"`
	Notes                  *string `json:"notes"`
}
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	customerID uuid.UUID,
	input CreateInput,
) (Contact, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Contact{}, errors.New("name is required")
	}

	method, err := normalizedContactMethod(input.PreferredContactMethod)
	if err != nil {
		return Contact{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Contact{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var customerExists bool

	err = tx.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM customers
			WHERE id = $1
			  AND archived_at IS NULL
		)
		`,
		customerID,
	).Scan(&customerExists)
	if err != nil {
		return Contact{}, err
	}

	if !customerExists {
		return Contact{}, ErrCustomerNotFound
	}

	if input.SiteID != nil {
		var siteCustomerID uuid.UUID

		err = tx.QueryRow(
			ctx,
			`
			SELECT customer_id
			FROM sites
			WHERE id = $1
			  AND archived_at IS NULL
			`,
			*input.SiteID,
		).Scan(&siteCustomerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Contact{}, ErrSiteNotFound
		}
		if err != nil {
			return Contact{}, err
		}

		if siteCustomerID != customerID {
			return Contact{}, ErrSiteDoesNotBelong
		}
	}

	const query = `
		INSERT INTO customer_contacts (
			customer_id,
			site_id,
			name,
			role_title,
			phone,
			email,
			preferred_contact_method,
			is_primary,
			notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING
			id,
			customer_id,
			site_id,
			name,
			role_title,
			phone,
			email,
			preferred_contact_method,
			is_primary,
			notes,
			created_at,
			updated_at
	`

	contact, err := scanContact(tx.QueryRow(
		ctx,
		query,
		customerID,
		input.SiteID,
		strings.TrimSpace(input.Name),
		normalizedOptionalString(input.RoleTitle),
		normalizedOptionalString(input.Phone),
		normalizedOptionalString(input.Email),
		method,
		input.IsPrimary,
		normalizedOptionalString(input.Notes),
	))
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Contact{}, ErrPrimaryContactExists
		}

		return Contact{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Contact{}, err
	}

	return contact, nil
}

func (r *Repository) Update(
	ctx context.Context,
	customerID uuid.UUID,
	contactID uuid.UUID,
	input UpdateInput,
) (Contact, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Contact{}, errors.New("name is required")
	}

	method, err := normalizedContactMethod(input.PreferredContactMethod)
	if err != nil {
		return Contact{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Contact{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var customerExists bool

	err = tx.QueryRow(
		ctx,
		`
                SELECT EXISTS (
                        SELECT 1
                        FROM customers
                        WHERE id = $1
                          AND archived_at IS NULL
                )
                `,
		customerID,
	).Scan(&customerExists)
	if err != nil {
		return Contact{}, err
	}

	if !customerExists {
		return Contact{}, ErrCustomerNotFound
	}

	if input.SiteID != nil {
		var siteCustomerID uuid.UUID

		err = tx.QueryRow(
			ctx,
			`
                        SELECT customer_id
                        FROM sites
                        WHERE id = $1
                          AND archived_at IS NULL
                        `,
			*input.SiteID,
		).Scan(&siteCustomerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Contact{}, ErrSiteNotFound
		}
		if err != nil {
			return Contact{}, err
		}

		if siteCustomerID != customerID {
			return Contact{}, ErrSiteDoesNotBelong
		}
	}

	const query = `
                UPDATE customer_contacts
                SET
                        site_id = $1,
                        name = $2,
                        role_title = $3,
                        phone = $4,
                        email = $5,
                        preferred_contact_method = $6,
                        is_primary = $7,
                        notes = $8,
                        updated_at = NOW()
                WHERE id = $9
                  AND customer_id = $10
                  AND archived_at IS NULL
                RETURNING
                        id,
                        customer_id,
                        site_id,
                        name,
                        role_title,
                        phone,
                        email,
                        preferred_contact_method,
                        is_primary,
                        notes,
                        created_at,
                        updated_at
        `

	contact, err := scanContact(tx.QueryRow(
		ctx,
		query,
		input.SiteID,
		strings.TrimSpace(input.Name),
		normalizedOptionalString(input.RoleTitle),
		normalizedOptionalString(input.Phone),
		normalizedOptionalString(input.Email),
		method,
		input.IsPrimary,
		normalizedOptionalString(input.Notes),
		contactID,
		customerID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return Contact{}, ErrContactNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Contact{}, ErrPrimaryContactExists
		}

		return Contact{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Contact{}, err
	}

	return contact, nil
}

func (r *Repository) ListByCustomer(
	ctx context.Context,
	customerID uuid.UUID,
) ([]Contact, error) {
	const customerExistsQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM customers
			WHERE id = $1
			  AND archived_at IS NULL
		)
	`

	var customerExists bool

	if err := r.db.QueryRow(ctx, customerExistsQuery, customerID).Scan(&customerExists); err != nil {
		return nil, err
	}

	if !customerExists {
		return nil, ErrCustomerNotFound
	}

	const query = `
		SELECT
			id,
			customer_id,
			site_id,
			name,
			role_title,
			phone,
			email,
			preferred_contact_method,
			is_primary,
			notes,
			created_at,
			updated_at
		FROM customer_contacts
		WHERE customer_id = $1
		  AND archived_at IS NULL
		ORDER BY
			CASE WHEN site_id IS NULL THEN 0 ELSE 1 END,
			CASE WHEN is_primary THEN 0 ELSE 1 END,
			name ASC,
			id ASC
	`

	rows, err := r.db.Query(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]Contact, 0)

	for rows.Next() {
		contact, err := scanContact(rows)
		if err != nil {
			return nil, err
		}

		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return contacts, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanContact(row rowScanner) (Contact, error) {
	var contact Contact

	err := row.Scan(
		&contact.ID,
		&contact.CustomerID,
		&contact.SiteID,
		&contact.Name,
		&contact.RoleTitle,
		&contact.Phone,
		&contact.Email,
		&contact.PreferredContactMethod,
		&contact.IsPrimary,
		&contact.Notes,
		&contact.CreatedAt,
		&contact.UpdatedAt,
	)
	if err != nil {
		return Contact{}, err
	}

	return contact, nil
}

func normalizedContactMethod(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}

	normalized := strings.ToUpper(strings.TrimSpace(*value))
	if normalized == "" {
		return nil, nil
	}

	switch normalized {
	case ContactMethodPhone, ContactMethodEmail, ContactMethodPersonal, ContactMethodOther:
		return &normalized, nil
	default:
		return nil, ErrInvalidContactMethod
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
