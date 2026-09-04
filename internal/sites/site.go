package sites

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCustomerNotFound = errors.New("customer not found")
var ErrSiteNotFound = errors.New("site not found")

type Site struct {
	ID             uuid.UUID `json:"id"`
	CustomerID     uuid.UUID `json:"customer_id"`
	Name           *string   `json:"name,omitempty"`
	NormalizedName *string   `json:"normalized_name,omitempty"`

	PostalCode     *string `json:"postal_code,omitempty"`
	City           string  `json:"city"`
	StreetName     *string `json:"street_name,omitempty"`
	StreetType     *string `json:"street_type,omitempty"`
	HouseNumber    *string `json:"house_number,omitempty"`
	AddressExtra   *string `json:"address_extra,omitempty"`
	AddressDisplay string  `json:"address_display"`

	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateInput struct {
	Name           *string `json:"name"`
	PostalCode     *string `json:"postal_code"`
	City           string  `json:"city"`
	StreetName     *string `json:"street_name"`
	StreetType     *string `json:"street_type"`
	HouseNumber    *string `json:"house_number"`
	AddressExtra   *string `json:"address_extra"`
	AddressDisplay string  `json:"address_display"`
	Notes          *string `json:"notes"`
}

type UpdateInput struct {
	Name           *string `json:"name"`
	PostalCode     *string `json:"postal_code"`
	City           string  `json:"city"`
	StreetName     *string `json:"street_name"`
	StreetType     *string `json:"street_type"`
	HouseNumber    *string `json:"house_number"`
	AddressExtra   *string `json:"address_extra"`
	AddressDisplay string  `json:"address_display"`
	Notes          *string `json:"notes"`
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
) (Site, error) {
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
		return Site{}, err
	}

	if !customerExists {
		return Site{}, ErrCustomerNotFound
	}

	const createQuery = `
		INSERT INTO sites (
			customer_id,
			name,
			normalized_name,
			postal_code,
			city,
			street_name,
			street_type,
			house_number,
			address_extra,
			address_display,
			notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING
			id,
			customer_id,
			name,
			normalized_name,
			postal_code,
			city,
			street_name,
			street_type,
			house_number,
			address_extra,
			address_display,
			notes,
			created_at,
			updated_at
	`

	name := optionalTrimmedString(input.Name)
	normalizedName := optionalNormalizedName(name)

	var site Site

	err := r.db.QueryRow(
		ctx,
		createQuery,
		customerID,
		name,
		normalizedName,
		optionalTrimmedString(input.PostalCode),
		strings.TrimSpace(input.City),
		optionalTrimmedString(input.StreetName),
		optionalTrimmedString(input.StreetType),
		optionalTrimmedString(input.HouseNumber),
		optionalTrimmedString(input.AddressExtra),
		strings.TrimSpace(input.AddressDisplay),
		optionalTrimmedString(input.Notes),
	).Scan(
		&site.ID,
		&site.CustomerID,
		&site.Name,
		&site.NormalizedName,
		&site.PostalCode,
		&site.City,
		&site.StreetName,
		&site.StreetType,
		&site.HouseNumber,
		&site.AddressExtra,
		&site.AddressDisplay,
		&site.Notes,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		return Site{}, err
	}

	return site, nil
}

func (r *Repository) ListByCustomer(
	ctx context.Context,
	customerID uuid.UUID,
) ([]Site, error) {
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

	const listQuery = `
		SELECT
			id,
			customer_id,
			name,
			normalized_name,
			postal_code,
			city,
			street_name,
			street_type,
			house_number,
			address_extra,
			address_display,
			notes,
			created_at,
			updated_at
		FROM sites
		WHERE customer_id = $1
		  AND archived_at IS NULL
		ORDER BY city ASC, address_display ASC, id ASC
	`

	rows, err := r.db.Query(ctx, listQuery, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	siteList := make([]Site, 0)

	for rows.Next() {
		site, err := scanSite(rows)
		if err != nil {
			return nil, err
		}

		siteList = append(siteList, site)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return siteList, nil
}

func (r *Repository) GetByID(ctx context.Context, siteID uuid.UUID) (Site, error) {
	const query = `
		SELECT
			id,
			customer_id,
			name,
			normalized_name,
			postal_code,
			city,
			street_name,
			street_type,
			house_number,
			address_extra,
			address_display,
			notes,
			created_at,
			updated_at
		FROM sites
		WHERE id = $1
		  AND archived_at IS NULL
	`

	site, err := scanSite(r.db.QueryRow(ctx, query, siteID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Site{}, ErrSiteNotFound
		}

		return Site{}, err
	}

	return site, nil
}

func (r *Repository) Update(
	ctx context.Context,
	siteID uuid.UUID,
	input UpdateInput,
) (Site, error) {
	const query = `
		UPDATE sites
		SET
			name = $2,
			normalized_name = $3,
			postal_code = $4,
			city = $5,
			street_name = $6,
			street_type = $7,
			house_number = $8,
			address_extra = $9,
			address_display = $10,
			notes = $11,
			updated_at = now()
		WHERE id = $1
			AND archived_at IS NULL
		RETURNING
			id,
			customer_id,
			name,
			normalized_name,
			postal_code,
			city,
			street_name,
			street_type,
			house_number,
			address_extra,
			address_display,
			notes,
			created_at,
			updated_at
	`

	name := optionalTrimmedString(input.Name)
	var site Site

	err := r.db.QueryRow(
		ctx,
		query,
		siteID,
		name,
		optionalNormalizedName(name),
		optionalTrimmedString(input.PostalCode),
		strings.TrimSpace(input.City),
		optionalTrimmedString(input.StreetName),
		optionalTrimmedString(input.StreetType),
		optionalTrimmedString(input.HouseNumber),
		optionalTrimmedString(input.AddressExtra),
		strings.TrimSpace(input.AddressDisplay),
		optionalTrimmedString(input.Notes),
	).Scan(
		&site.ID,
		&site.CustomerID,
		&site.Name,
		&site.NormalizedName,
		&site.PostalCode,
		&site.City,
		&site.StreetName,
		&site.StreetType,
		&site.HouseNumber,
		&site.AddressExtra,
		&site.AddressDisplay,
		&site.Notes,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Site{}, ErrSiteNotFound
		}
		return Site{}, err
	}

	return site, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSite(row rowScanner) (Site, error) {
	var site Site

	err := row.Scan(
		&site.ID,
		&site.CustomerID,
		&site.Name,
		&site.NormalizedName,
		&site.PostalCode,
		&site.City,
		&site.StreetName,
		&site.StreetType,
		&site.HouseNumber,
		&site.AddressExtra,
		&site.AddressDisplay,
		&site.Notes,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		return Site{}, err
	}

	return site, nil
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

func optionalNormalizedName(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := normalizeText(*value)
	return &normalized
}

func normalizeText(value string) string {
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
