package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hajdurenato/fredi-api/internal/extinguishers"
)

type extinguisherHandler struct {
	repository *extinguishers.Repository
}

func NewExtinguisherHandler(repository *extinguishers.Repository) *extinguisherHandler {
	return &extinguisherHandler{repository: repository}
}

func (h *extinguisherHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input extinguishers.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.ExtinguisherTypeCode) == "" {
		writeJSONError(w, http.StatusBadRequest, "extinguisher_type_code is required")
		return
	}

	if strings.TrimSpace(input.ExtinguisherTypeDisplay) == "" {
		writeJSONError(w, http.StatusBadRequest, "extinguisher_type_display is required")
		return
	}

	if !extinguishers.IsValidLifecycleStatus(input.LifecycleStatus) {
		writeJSONError(w, http.StatusBadRequest, "invalid lifecycle_status")
		return
	}

	if input.CapacityKG != nil && *input.CapacityKG <= 0 {
		writeJSONError(w, http.StatusBadRequest, "capacity_kg must be greater than zero")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	extinguisher, err := h.repository.Create(ctx, input)
	if err != nil {
		if errors.Is(err, extinguishers.ErrOKFNumberAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "an active extinguisher with this OKF number already exists")
			return
		}

		if err.Error() == "invalid date; expected YYYY-MM-DD" {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to create extinguisher")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/extinguishers/"+extinguisher.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(extinguisher)
}

func (h *extinguisherHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	extinguisherList, err := h.repository.List(ctx)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list extinguishers")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(extinguisherList)
}

func (h *extinguisherHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	extinguisherID, ok := pathUUID(w, r, "extinguisherID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	extinguisher, err := h.repository.GetByID(ctx, extinguisherID)
	if err != nil {
		if errors.Is(err, extinguishers.ErrExtinguisherNotFound) {
			writeJSONError(w, http.StatusNotFound, "extinguisher not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to get extinguisher")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(extinguisher)
}
