package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hajdurenato/fredi-api/internal/equipmentlocations"
)

type equipmentLocationHandler struct {
	repository *equipmentlocations.Repository
}

func NewEquipmentLocationHandler(
	repository *equipmentlocations.Repository,
) *equipmentLocationHandler {
	return &equipmentLocationHandler{repository: repository}
}

func (h *equipmentLocationHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	siteID, ok := pathUUID(w, r, "siteID")
	if !ok {
		return
	}

	var input equipmentlocations.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.Description) == "" {
		writeJSONError(w, http.StatusBadRequest, "description is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	location, err := h.repository.Create(ctx, siteID, input)
	if err != nil {
		switch {
		case errors.Is(err, equipmentlocations.ErrSiteNotFound):
			writeJSONError(w, http.StatusNotFound, "site not found")
		case errors.Is(err, equipmentlocations.ErrLocationCodeAlreadyExists):
			writeJSONError(
				w,
				http.StatusConflict,
				"an active equipment location with this code already exists at this site",
			)
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to create equipment location")
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(
		"Location",
		"/api/v1/equipment-locations/"+location.ID.String(),
	)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(location)
}

func (h *equipmentLocationHandler) ListBySite(
	w http.ResponseWriter,
	r *http.Request,
) {
	siteID, ok := pathUUID(w, r, "siteID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	locationList, err := h.repository.ListBySite(ctx, siteID)
	if err != nil {
		if errors.Is(err, equipmentlocations.ErrSiteNotFound) {
			writeJSONError(w, http.StatusNotFound, "site not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to list equipment locations")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(locationList)
}

func (h *equipmentLocationHandler) GetByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	locationID, ok := pathUUID(w, r, "locationID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	location, err := h.repository.GetByID(ctx, locationID)
	if err != nil {
		if errors.Is(err, equipmentlocations.ErrLocationNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment location not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to get equipment location")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(location)
}
