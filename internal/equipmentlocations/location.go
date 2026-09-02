package equipmentlocations

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

var ErrSiteNotFound = errors.New("site not found")
var ErrLocationNotFound = errors.New("equipment location not found")
var ErrLocationCodeAlreadyExists = errors.New("location code already exists")

type Location struct {
	ID                    uuid.UUID `json:"id"`
	SiteID                uuid.UUID `json:"site_id"`
	LocationCode          *string   `json:"location_code,omitempty"`
	Description           string    `json:"description"`
	NormalizedDescription string    `json:"normalized_description"`
	FloorOrZone           *string   `json:"floor_or_zone,omitempty"`
	SortOrder             int       `json:"sort_order"`
	Notes                 *string   `json:"notes,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type CreateInput struct {
	LocationCode *string `json:"location_code"`
	Description  string  `json:"description"`
	FloorOrZone  *string `json:"floor_or_zone"`
	SortOrder    *int    `json:"sort_order"`
	Notes        *string `json:"notes"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	siteID uuid.UUID,
	input CreateInput,
) (Location, error) {
	const siteExistsQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM sites
			WHERE id = $1
			  AND archived_at IS NULL
		)
	`

	var siteExists bool
	if err := r.db.QueryRow(ctx, siteExistsQuery, siteID).Scan(&siteExists); err != nil {
		return Location{}, err
	}

	if !siteExists {
		return Location{}, ErrSiteNotFound
	}

	const createQuery = `
		INSERT INTO fire_equipment_locations (
			site_id,
			location_code,
			description,
			normalized_description,
			floor_or_zone,
			sort_order,
			notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			site_id,
			location_code,
			description,
			normalized_description,
			floor_or_zone,
			sort_order,
			notes,
			created_at,
			updated_at
	`

	description := strings.TrimSpace(input.Description)

	var location Location
	err := r.db.QueryRow(
		ctx,
		createQuery,
		siteID,
		optionalTrimmedString(input.LocationCode),
		description,
		normalizeText(description),
		optionalTrimmedString(input.FloorOrZone),
		optionalInt(input.SortOrder),
		optionalTrimmedString(input.Notes),
	).Scan(
		&location.ID,
		&location.SiteID,
		&location.LocationCode,
		&location.Description,
		&location.NormalizedDescription,
		&location.FloorOrZone,
		&location.SortOrder,
		&location.Notes,
		&location.CreatedAt,
		&location.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Location{}, ErrLocationCodeAlreadyExists
		}

		return Location{}, err
	}

	return location, nil
}

func (r *Repository) ListBySite(
	ctx context.Context,
	siteID uuid.UUID,
) ([]Location, error) {
	const siteExistsQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM sites
			WHERE id = $1
			  AND archived_at IS NULL
		)
	`

	var siteExists bool
	if err := r.db.QueryRow(ctx, siteExistsQuery, siteID).Scan(&siteExists); err != nil {
		return nil, err
	}

	if !siteExists {
		return nil, ErrSiteNotFound
	}

	const listQuery = `
		SELECT
			id,
			site_id,
			location_code,
			description,
			normalized_description,
			floor_or_zone,
			sort_order,
			notes,
			created_at,
			updated_at
		FROM fire_equipment_locations
		WHERE site_id = $1
		  AND archived_at IS NULL
		ORDER BY sort_order ASC, description ASC, id ASC
	`

	rows, err := r.db.Query(ctx, listQuery, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locationList := make([]Location, 0)
	for rows.Next() {
		location, err := scanLocation(rows)
		if err != nil {
			return nil, err
		}

		locationList = append(locationList, location)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return locationList, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	locationID uuid.UUID,
) (Location, error) {
	const query = `
		SELECT
			id,
			site_id,
			location_code,
			description,
			normalized_description,
			floor_or_zone,
			sort_order,
			notes,
			created_at,
			updated_at
		FROM fire_equipment_locations
		WHERE id = $1
		  AND archived_at IS NULL
	`

	location, err := scanLocation(r.db.QueryRow(ctx, query, locationID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Location{}, ErrLocationNotFound
		}

		return Location{}, err
	}

	return location, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLocation(row rowScanner) (Location, error) {
	var location Location

	err := row.Scan(
		&location.ID,
		&location.SiteID,
		&location.LocationCode,
		&location.Description,
		&location.NormalizedDescription,
		&location.FloorOrZone,
		&location.SortOrder,
		&location.Notes,
		&location.CreatedAt,
		&location.UpdatedAt,
	)
	if err != nil {
		return Location{}, err
	}

	return location, nil
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

func optionalInt(value *int) int {
	if value == nil {
		return 0
	}

	return *value
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
