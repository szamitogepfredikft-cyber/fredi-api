package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/inspectors"
)

type inspectorHandler struct {
	repository *inspectors.Repository
}

func NewInspectorHandler(
	repository *inspectors.Repository,
) *inspectorHandler {
	return &inspectorHandler{repository: repository}
}

func (h *inspectorHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	input := inspectors.ListInput{
		View:  r.URL.Query().Get("view"),
		Query: strings.TrimSpace(r.URL.Query().Get("q")),
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	items, err := h.repository.List(ctx, input)
	if err != nil {
		if err.Error() == "invalid inspector list view" {
			writeJSONError(
				w,
				http.StatusBadRequest,
				"view must be one of: active, archived",
			)
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to list inspectors",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (h *inspectorHandler) GetByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	inspector, err := h.repository.GetByID(ctx, inspectorID)
	if err != nil {
		if errors.Is(err, inspectors.ErrInspectorNotFound) {
			writeJSONError(w, http.StatusNotFound, "inspector not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to get inspector",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inspector)
}

func (h *inspectorHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input inspectors.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.Name) == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	inspector, err := h.repository.Create(ctx, input)
	if err != nil {
		if err.Error() == "inspector name is required" {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to create inspector",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/inspectors/"+inspector.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(inspector)
}

func (h *inspectorHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	var input inspectors.UpdateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if input.Name == nil || strings.TrimSpace(*input.Name) == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	inspector, err := h.repository.Update(ctx, inspectorID, input)
	if err != nil {
		switch {
		case errors.Is(err, inspectors.ErrInspectorNotFound):
			writeJSONError(w, http.StatusNotFound, "inspector not found")
		case err.Error() == "inspector name is required":
			writeJSONError(w, http.StatusBadRequest, "name is required")
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to update inspector",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inspector)
}

func (h *inspectorHandler) Archive(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	inspector, err := h.repository.Archive(ctx, inspectorID)
	if err != nil {
		if errors.Is(err, inspectors.ErrInspectorNotFound) {
			writeJSONError(w, http.StatusNotFound, "inspector not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to archive inspector",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inspector)
}

func (h *inspectorHandler) Restore(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	inspector, err := h.repository.Restore(ctx, inspectorID)
	if err != nil {
		if errors.Is(err, inspectors.ErrInspectorNotFound) {
			writeJSONError(w, http.StatusNotFound, "inspector not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to restore inspector",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inspector)
}

func (h *inspectorHandler) CreateCertificate(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	var input inspectors.CreateCertificateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.CertificateNumber) == "" {
		writeJSONError(w, http.StatusBadRequest, "certificate_number is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	certificate, err := h.repository.CreateCertificate(ctx, inspectorID, input)
	if err != nil {
		switch {
		case errors.Is(err, inspectors.ErrInspectorNotFound):
			writeJSONError(w, http.StatusNotFound, "active inspector not found")
		case errors.Is(err, inspectors.ErrDuplicateCertificate):
			writeJSONError(
				w,
				http.StatusConflict,
				"an active inspector certificate already uses this number",
			)
		case err.Error() == "certificate number is required":
			writeJSONError(w, http.StatusBadRequest, "certificate_number is required")
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to create inspector certificate",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(certificate)
}

func (h *inspectorHandler) UpdateCertificate(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	certificateID, ok := pathUUID(w, r, "certificateID")
	if !ok {
		return
	}

	var input inspectors.UpdateCertificateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if input.CertificateNumber == nil ||
		strings.TrimSpace(*input.CertificateNumber) == "" {
		writeJSONError(w, http.StatusBadRequest, "certificate_number is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	certificate, err := h.repository.UpdateCertificate(
		ctx,
		inspectorID,
		certificateID,
		input,
	)
	if err != nil {
		switch {
		case errors.Is(err, inspectors.ErrCertificateNotFound):
			writeJSONError(w, http.StatusNotFound, "inspector certificate not found")
		case errors.Is(err, inspectors.ErrDuplicateCertificate):
			writeJSONError(
				w,
				http.StatusConflict,
				"an active inspector certificate already uses this number",
			)
		case err.Error() == "certificate number is required":
			writeJSONError(w, http.StatusBadRequest, "certificate_number is required")
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to update inspector certificate",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(certificate)
}

func (h *inspectorHandler) ArchiveCertificate(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	certificateID, ok := pathUUID(w, r, "certificateID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	certificate, err := h.repository.ArchiveCertificate(
		ctx,
		inspectorID,
		certificateID,
	)
	if err != nil {
		if errors.Is(err, inspectors.ErrCertificateNotFound) {
			writeJSONError(w, http.StatusNotFound, "inspector certificate not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to archive inspector certificate",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(certificate)
}

func (h *inspectorHandler) RestoreCertificate(
	w http.ResponseWriter,
	r *http.Request,
) {
	inspectorID, ok := pathUUID(w, r, "inspectorID")
	if !ok {
		return
	}

	certificateID, ok := pathUUID(w, r, "certificateID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	certificate, err := h.repository.RestoreCertificate(
		ctx,
		inspectorID,
		certificateID,
	)
	if err != nil {
		switch {
		case errors.Is(err, inspectors.ErrCertificateNotFound):
			writeJSONError(w, http.StatusNotFound, "inspector certificate not found")
		case errors.Is(err, inspectors.ErrDuplicateCertificate):
			writeJSONError(
				w,
				http.StatusConflict,
				"an active inspector certificate already uses this number",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to restore inspector certificate",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(certificate)
}

var _ = uuid.Nil
