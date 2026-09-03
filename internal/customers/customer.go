package customers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTaxNumberAlreadyExists = errors.New("tax number already exists")

type Customer struct {
	ID                    uuid.UUID `json:"id"`
	Name                  string    `json:"name"`
	NormalizedName        string    `json:"normalized_name"`
	TaxNumber             *string   `json:"tax_number,omitempty"`
	BillingAddressDisplay *string   `json:"billing_address_display,omitempty"`
	Notes                 *string   `json:"notes,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type CreateInput struct {
	Name                  string  `json:"name"`
	TaxNumber             *string `json:"tax_number"`
	BillingAddressDisplay *string `json:"billing_address_display"`
	Notes                 *string `json:"notes"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (Customer, error) {
	const query = `
		INSERT INTO customers (
			name,
			normalized_name,
			tax_number,
			billing_address_display,
			notes
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			name,
			normalized_name,
			tax_number,
			billing_address_display,
			notes,
			created_at,
			updated_at
	`

	name := strings.TrimSpace(input.Name)
	normalizedName := normalizeName(name)

	var customer Customer

	err := r.db.QueryRow(
		ctx,
		query,
		name,
		normalizedName,
		optionalTrimmedString(input.TaxNumber),
		optionalTrimmedString(input.BillingAddressDisplay),
		optionalTrimmedString(input.Notes),
	).Scan(
		&customer.ID,
		&customer.Name,
		&customer.NormalizedName,
		&customer.TaxNumber,
		&customer.BillingAddressDisplay,
		&customer.Notes,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Customer{}, ErrTaxNumberAlreadyExists
		}

		return Customer{}, err
	}

	return customer, nil
}

func (r *Repository) List(ctx context.Context) ([]Customer, error) {
	const query = `
		SELECT
			id,
			name,
			normalized_name,
			tax_number,
			billing_address_display,
			notes,
			created_at,
			updated_at
		FROM customers
		WHERE archived_at IS NULL
		ORDER BY normalized_name ASC, id ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customers := make([]Customer, 0)

	for rows.Next() {
		var customer Customer

		if err := rows.Scan(
			&customer.ID,
			&customer.Name,
			&customer.NormalizedName,
			&customer.TaxNumber,
			&customer.BillingAddressDisplay,
			&customer.Notes,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		); err != nil {
			return nil, err
		}

		customers = append(customers, customer)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return customers, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	customerID uuid.UUID,
) (Customer, error) {
	const query = `
		SELECT
			id,
			name,
			normalized_name,
			tax_number,
			billing_address_display,
			notes,
			created_at,
			updated_at
		FROM customers
		WHERE id = $1
			AND archived_at IS NULL
	`

	var customer Customer

	err := r.db.QueryRow(ctx, query, customerID).Scan(
		&customer.ID,
		&customer.Name,
		&customer.NormalizedName,
		&customer.TaxNumber,
		&customer.BillingAddressDisplay,
		&customer.Notes,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		return Customer{}, err
	}

	return customer, nil
}

func normalizeName(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))

	replacer := strings.NewReplacer(
		"Á", "A",
		"É", "E",
		"Í", "I",
		"Ó", "O",
		"Ö", "O",
		"Ő", "O",
		"Ú", "U",
		"Ü", "U",
		"Ű", "U",
		" ", "",
		".", "",
		",", "",
		"-", "",
	)

	return replacer.Replace(value)
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
