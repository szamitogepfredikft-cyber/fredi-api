package inspectors

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInspectorNotFound    = errors.New("inspector not found")
	ErrCertificateNotFound  = errors.New("inspector certificate not found")
	ErrDuplicateCertificate = errors.New("active certificate number already exists")
)

type Inspector struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Phone        *string       `json:"phone,omitempty"`
	Email        *string       `json:"email,omitempty"`
	Notes        *string       `json:"notes,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	ArchivedAt   *time.Time    `json:"archived_at,omitempty"`
	Certificates []Certificate `json:"certificates"`
}

type Certificate struct {
	ID                uuid.UUID  `json:"id"`
	InspectorID       uuid.UUID  `json:"inspector_id"`
	CertificateNumber string     `json:"certificate_number"`
	Notes             *string    `json:"notes,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ArchivedAt        *time.Time `json:"archived_at,omitempty"`
}

type ListInput struct {
	View  string
	Query string
}

type CreateInput struct {
	Name  string  `json:"name"`
	Phone *string `json:"phone"`
	Email *string `json:"email"`
	Notes *string `json:"notes"`
}

type UpdateInput struct {
	Name  *string `json:"name"`
	Phone *string `json:"phone"`
	Email *string `json:"email"`
	Notes *string `json:"notes"`
}

type CreateCertificateInput struct {
	CertificateNumber string  `json:"certificate_number"`
	Notes             *string `json:"notes"`
}

type UpdateCertificateInput struct {
	CertificateNumber *string `json:"certificate_number"`
	Notes             *string `json:"notes"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(
	ctx context.Context,
	input ListInput,
) ([]Inspector, error) {
	view := input.View
	if view == "" {
		view = "active"
	}

	if view != "active" && view != "archived" {
		return nil, errors.New("invalid inspector list view")
	}

	const listQuery = `
		SELECT
			i.id,
			i.name,
			i.phone,
			i.email,
			i.notes,
			i.created_at,
			i.updated_at,
			i.archived_at
		FROM inspectors i
		WHERE (
			($1 = 'active' AND i.archived_at IS NULL)
			OR ($1 = 'archived' AND i.archived_at IS NOT NULL)
		)
		AND (
			$2 = ''
			OR i.name ILIKE '%' || $2 || '%'
			OR COALESCE(i.phone, '') ILIKE '%' || $2 || '%'
			OR COALESCE(i.email, '') ILIKE '%' || $2 || '%'
			OR EXISTS (
				SELECT 1
				FROM inspector_certificates c
				WHERE c.inspector_id = i.id
					AND c.certificate_number ILIKE '%' || $2 || '%'
			)
		)
		ORDER BY i.name, i.created_at
	`

	rows, err := r.db.Query(ctx, listQuery, view, input.Query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Inspector, 0)

	for rows.Next() {
		var inspector Inspector

		if err := rows.Scan(
			&inspector.ID,
			&inspector.Name,
			&inspector.Phone,
			&inspector.Email,
			&inspector.Notes,
			&inspector.CreatedAt,
			&inspector.UpdatedAt,
			&inspector.ArchivedAt,
		); err != nil {
			return nil, err
		}

		certificates, err := r.listCertificates(ctx, inspector.ID, view)
		if err != nil {
			return nil, err
		}

		inspector.Certificates = certificates
		items = append(items, inspector)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) listCertificates(
	ctx context.Context,
	inspectorID uuid.UUID,
	view string,
) ([]Certificate, error) {
	const listCertificatesQuery = `
		SELECT
			id,
			inspector_id,
			certificate_number,
			notes,
			created_at,
			updated_at,
			archived_at
		FROM inspector_certificates
		WHERE inspector_id = $1
		AND (
			$2 = 'archived'
			OR archived_at IS NULL
		)
		ORDER BY certificate_number, created_at
	`

	rows, err := r.db.Query(
		ctx,
		listCertificatesQuery,
		inspectorID,
		view,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Certificate, 0)

	for rows.Next() {
		var certificate Certificate

		if err := rows.Scan(
			&certificate.ID,
			&certificate.InspectorID,
			&certificate.CertificateNumber,
			&certificate.Notes,
			&certificate.CreatedAt,
			&certificate.UpdatedAt,
			&certificate.ArchivedAt,
		); err != nil {
			return nil, err
		}

		items = append(items, certificate)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	inspectorID uuid.UUID,
) (Inspector, error) {
	const getByIDQuery = `
		SELECT
			id,
			name,
			phone,
			email,
			notes,
			created_at,
			updated_at,
			archived_at
		FROM inspectors
		WHERE id = $1
	`

	var inspector Inspector

	err := r.db.QueryRow(ctx, getByIDQuery, inspectorID).Scan(
		&inspector.ID,
		&inspector.Name,
		&inspector.Phone,
		&inspector.Email,
		&inspector.Notes,
		&inspector.CreatedAt,
		&inspector.UpdatedAt,
		&inspector.ArchivedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Inspector{}, ErrInspectorNotFound
		}

		return Inspector{}, err
	}

	certificates, err := r.listCertificates(ctx, inspector.ID, "archived")
	if err != nil {
		return Inspector{}, err
	}

	inspector.Certificates = certificates

	return inspector, nil
}

func (r *Repository) Create(
	ctx context.Context,
	input CreateInput,
) (Inspector, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Inspector{}, errors.New("inspector name is required")
	}

	const createQuery = `
		INSERT INTO inspectors (
			name,
			phone,
			email,
			notes
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			name,
			phone,
			email,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var inspector Inspector

	err := r.db.QueryRow(
		ctx,
		createQuery,
		name,
		normalizeOptionalString(input.Phone),
		normalizeOptionalString(input.Email),
		normalizeOptionalString(input.Notes),
	).Scan(
		&inspector.ID,
		&inspector.Name,
		&inspector.Phone,
		&inspector.Email,
		&inspector.Notes,
		&inspector.CreatedAt,
		&inspector.UpdatedAt,
		&inspector.ArchivedAt,
	)
	if err != nil {
		return Inspector{}, err
	}

	inspector.Certificates = make([]Certificate, 0)

	return inspector, nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}

	return &normalized
}

func (r *Repository) Update(
	ctx context.Context,
	inspectorID uuid.UUID,
	input UpdateInput,
) (Inspector, error) {
	if input.Name == nil {
		return Inspector{}, errors.New("inspector name is required")
	}

	name := strings.TrimSpace(*input.Name)
	if name == "" {
		return Inspector{}, errors.New("inspector name is required")
	}

	const updateQuery = `
		UPDATE inspectors
		SET
			name = $2,
			phone = $3,
			email = $4,
			notes = $5,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			phone,
			email,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var inspector Inspector

	err := r.db.QueryRow(
		ctx,
		updateQuery,
		inspectorID,
		name,
		normalizeOptionalString(input.Phone),
		normalizeOptionalString(input.Email),
		normalizeOptionalString(input.Notes),
	).Scan(
		&inspector.ID,
		&inspector.Name,
		&inspector.Phone,
		&inspector.Email,
		&inspector.Notes,
		&inspector.CreatedAt,
		&inspector.UpdatedAt,
		&inspector.ArchivedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Inspector{}, ErrInspectorNotFound
		}

		return Inspector{}, err
	}

	certificates, err := r.listCertificates(ctx, inspector.ID, "archived")
	if err != nil {
		return Inspector{}, err
	}

	inspector.Certificates = certificates

	return inspector, nil
}

func (r *Repository) Archive(
	ctx context.Context,
	inspectorID uuid.UUID,
) (Inspector, error) {
	transaction, err := r.db.Begin(ctx)
	if err != nil {
		return Inspector{}, err
	}
	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	const archiveInspectorQuery = `
		UPDATE inspectors
		SET
			archived_at = COALESCE(archived_at, NOW()),
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			phone,
			email,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var inspector Inspector

	err = transaction.QueryRow(
		ctx,
		archiveInspectorQuery,
		inspectorID,
	).Scan(
		&inspector.ID,
		&inspector.Name,
		&inspector.Phone,
		&inspector.Email,
		&inspector.Notes,
		&inspector.CreatedAt,
		&inspector.UpdatedAt,
		&inspector.ArchivedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Inspector{}, ErrInspectorNotFound
		}

		return Inspector{}, err
	}

	const archiveCertificatesQuery = `
		UPDATE inspector_certificates
		SET
			archived_at = COALESCE(archived_at, NOW()),
			updated_at = NOW()
		WHERE inspector_id = $1
		  AND archived_at IS NULL
	`

	if _, err := transaction.Exec(ctx, archiveCertificatesQuery, inspectorID); err != nil {
		return Inspector{}, err
	}

	if err := transaction.Commit(ctx); err != nil {
		return Inspector{}, err
	}

	certificates, err := r.listCertificates(ctx, inspector.ID, "archived")
	if err != nil {
		return Inspector{}, err
	}

	inspector.Certificates = certificates

	return inspector, nil
}

func (r *Repository) Restore(
	ctx context.Context,
	inspectorID uuid.UUID,
) (Inspector, error) {
	const restoreQuery = `
		UPDATE inspectors
		SET
			archived_at = NULL,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			phone,
			email,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var inspector Inspector

	err := r.db.QueryRow(ctx, restoreQuery, inspectorID).Scan(
		&inspector.ID,
		&inspector.Name,
		&inspector.Phone,
		&inspector.Email,
		&inspector.Notes,
		&inspector.CreatedAt,
		&inspector.UpdatedAt,
		&inspector.ArchivedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Inspector{}, ErrInspectorNotFound
		}

		return Inspector{}, err
	}

	certificates, err := r.listCertificates(ctx, inspector.ID, "archived")
	if err != nil {
		return Inspector{}, err
	}

	inspector.Certificates = certificates

	return inspector, nil
}

func (r *Repository) CreateCertificate(
	ctx context.Context,
	inspectorID uuid.UUID,
	input CreateCertificateInput,
) (Certificate, error) {
	certificateNumber := strings.TrimSpace(input.CertificateNumber)
	if certificateNumber == "" {
		return Certificate{}, errors.New("certificate number is required")
	}

	const createCertificateQuery = `
		INSERT INTO inspector_certificates (
			inspector_id,
			certificate_number,
			notes
		)
		SELECT
			$1,
			$2,
			$3
		WHERE EXISTS (
			SELECT 1
			FROM inspectors
			WHERE id = $1
			  AND archived_at IS NULL
		)
		RETURNING
			id,
			inspector_id,
			certificate_number,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var certificate Certificate

	err := r.db.QueryRow(
		ctx,
		createCertificateQuery,
		inspectorID,
		certificateNumber,
		normalizeOptionalString(input.Notes),
	).Scan(
		&certificate.ID,
		&certificate.InspectorID,
		&certificate.CertificateNumber,
		&certificate.Notes,
		&certificate.CreatedAt,
		&certificate.UpdatedAt,
		&certificate.ArchivedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Certificate{}, ErrInspectorNotFound
		case isUniqueViolation(err):
			return Certificate{}, ErrDuplicateCertificate
		default:
			return Certificate{}, err
		}
	}

	return certificate, nil
}

func (r *Repository) UpdateCertificate(
	ctx context.Context,
	inspectorID uuid.UUID,
	certificateID uuid.UUID,
	input UpdateCertificateInput,
) (Certificate, error) {
	if input.CertificateNumber == nil {
		return Certificate{}, errors.New("certificate number is required")
	}

	certificateNumber := strings.TrimSpace(*input.CertificateNumber)
	if certificateNumber == "" {
		return Certificate{}, errors.New("certificate number is required")
	}

	const updateCertificateQuery = `
		UPDATE inspector_certificates
		SET
			certificate_number = $3,
			notes = $4,
			updated_at = NOW()
		WHERE id = $2
		  AND inspector_id = $1
		RETURNING
			id,
			inspector_id,
			certificate_number,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var certificate Certificate

	err := r.db.QueryRow(
		ctx,
		updateCertificateQuery,
		inspectorID,
		certificateID,
		certificateNumber,
		normalizeOptionalString(input.Notes),
	).Scan(
		&certificate.ID,
		&certificate.InspectorID,
		&certificate.CertificateNumber,
		&certificate.Notes,
		&certificate.CreatedAt,
		&certificate.UpdatedAt,
		&certificate.ArchivedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Certificate{}, ErrCertificateNotFound
		case isUniqueViolation(err):
			return Certificate{}, ErrDuplicateCertificate
		default:
			return Certificate{}, err
		}
	}

	return certificate, nil
}

func (r *Repository) ArchiveCertificate(
	ctx context.Context,
	inspectorID uuid.UUID,
	certificateID uuid.UUID,
) (Certificate, error) {
	const archiveCertificateQuery = `
		UPDATE inspector_certificates
		SET
			archived_at = COALESCE(archived_at, NOW()),
			updated_at = NOW()
		WHERE id = $2
		  AND inspector_id = $1
		RETURNING
			id,
			inspector_id,
			certificate_number,
			notes,
			created_at,
			updated_at,
			archived_at
	`

	var certificate Certificate

	err := r.db.QueryRow(
		ctx,
		archiveCertificateQuery,
		inspectorID,
		certificateID,
	).Scan(
		&certificate.ID,
		&certificate.InspectorID,
		&certificate.CertificateNumber,
		&certificate.Notes,
		&certificate.CreatedAt,
		&certificate.UpdatedAt,
		&certificate.ArchivedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Certificate{}, ErrCertificateNotFound
		}

		return Certificate{}, err
	}

	return certificate, nil
}

func (r *Repository) RestoreCertificate(
	ctx context.Context,
	inspectorID uuid.UUID,
	certificateID uuid.UUID,
) (Certificate, error) {
	const restoreCertificateQuery = `
		UPDATE inspector_certificates c
		SET
			archived_at = NULL,
			updated_at = NOW()
		WHERE c.id = $2
		  AND c.inspector_id = $1
		  AND EXISTS (
			SELECT 1
			FROM inspectors i
			WHERE i.id = c.inspector_id
			  AND i.archived_at IS NULL
		  )
		RETURNING
			c.id,
			c.inspector_id,
			c.certificate_number,
			c.notes,
			c.created_at,
			c.updated_at,
			c.archived_at
	`

	var certificate Certificate

	err := r.db.QueryRow(
		ctx,
		restoreCertificateQuery,
		inspectorID,
		certificateID,
	).Scan(
		&certificate.ID,
		&certificate.InspectorID,
		&certificate.CertificateNumber,
		&certificate.Notes,
		&certificate.CreatedAt,
		&certificate.UpdatedAt,
		&certificate.ArchivedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Certificate{}, ErrCertificateNotFound
		case isUniqueViolation(err):
			return Certificate{}, ErrDuplicateCertificate
		default:
			return Certificate{}, err
		}
	}

	return certificate, nil
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError

	return errors.As(err, &pgError) && pgError.Code == "23505"
}
