package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/sites"
)

type siteHandler struct {
	repository *sites.Repository
}

func NewSiteHandler(repository *sites.Repository) *siteHandler {
	return &siteHandler{repository: repository}
}

func (h *siteHandler) Create(w http.ResponseWriter, r *http.Request) {
	customerID, ok := pathUUID(w, r, "customerID")
	if !ok {
		return
	}

	var input sites.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.City) == "" {
		writeJSONError(w, http.StatusBadRequest, "city is required")
		return
	}

	if strings.TrimSpace(input.AddressDisplay) == "" {
		writeJSONError(w, http.StatusBadRequest, "address_display is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	site, err := h.repository.Create(ctx, customerID, input)
	if err != nil {
		if errors.Is(err, sites.ErrCustomerNotFound) {
			writeJSONError(w, http.StatusNotFound, "customer not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to create site")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/sites/"+site.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(site)
}

func (h *siteHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	customerID, ok := pathUUID(w, r, "customerID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	siteList, err := h.repository.ListByCustomer(ctx, customerID)
	if err != nil {
		if errors.Is(err, sites.ErrCustomerNotFound) {
			writeJSONError(w, http.StatusNotFound, "customer not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to list sites")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(siteList)
}

func (h *siteHandler) Update(w http.ResponseWriter, r *http.Request) {
	siteID, ok := pathUUID(w, r, "siteID")
	if !ok {
		return
	}

	var input sites.UpdateInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.City) == "" {
		writeJSONError(w, http.StatusBadRequest, "city is required")
		return
	}

	if strings.TrimSpace(input.AddressDisplay) == "" {
		writeJSONError(w, http.StatusBadRequest, "address_display is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	site, err := h.repository.Update(ctx, siteID, input)
	if err != nil {
		if errors.Is(err, sites.ErrSiteNotFound) {
			writeJSONError(w, http.StatusNotFound, "site not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to update site")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(site)
}

func (h *siteHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	siteID, ok := pathUUID(w, r, "siteID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	site, err := h.repository.GetByID(ctx, siteID)
	if err != nil {
		if errors.Is(err, sites.ErrSiteNotFound) {
			writeJSONError(w, http.StatusNotFound, "site not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to get site")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(site)
}

func pathUUID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	rawID := chi.URLParam(r, key)

	id, err := uuid.Parse(rawID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid "+key)
		return uuid.Nil, false
	}

	return id, true
}
