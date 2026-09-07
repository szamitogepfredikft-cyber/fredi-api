package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hajdurenato/fredi-api/internal/fireinspectionrows"
)

type fireInspectionRowHandler struct {
	repository *fireinspectionrows.Repository
}

func NewFireInspectionRowHandler(
	repository *fireinspectionrows.Repository,
) *fireInspectionRowHandler {
	return &fireInspectionRowHandler{repository: repository}
}

func (h *fireInspectionRowHandler) Initialize(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	rowList, created, err := h.repository.InitializeForJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, fireinspectionrows.ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to initialize fire inspection job rows",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if created {
		w.WriteHeader(http.StatusCreated)
	}

	_ = json.NewEncoder(w).Encode(rowList)
}

func (h *fireInspectionRowHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	var input fireinspectionrows.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	input.LocationCode = strings.TrimSpace(input.LocationCode)
	input.LocationName = strings.TrimSpace(input.LocationName)
	input.ExtinguisherType = strings.TrimSpace(input.ExtinguisherType)
	input.OKFNumber = strings.TrimSpace(input.OKFNumber)

	if input.LocationName == "" {
		writeJSONError(w, http.StatusBadRequest, "location_name is required")
		return
	}

	if input.ExtinguisherType == "" {
		writeJSONError(w, http.StatusBadRequest, "extinguisher_type is required")
		return
	}

	if input.CapacityKG <= 0 {
		writeJSONError(w, http.StatusBadRequest, "capacity_kg must be greater than zero")
		return
	}

	if input.OKFNumber == "" {
		writeJSONError(w, http.StatusBadRequest, "okf_number is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	row, err := h.repository.CreateForJob(ctx, jobID, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionrows.ErrJobNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
		case errors.Is(err, fireinspectionrows.ErrJobNotEditable):
			writeJSONError(w, http.StatusConflict, "fire inspection job is not editable")
		case errors.Is(err, fireinspectionrows.ErrLocationCodeAlreadyExists):
			writeJSONError(w, http.StatusConflict, "an equipment location with this code already exists at this site")
		case errors.Is(err, fireinspectionrows.ErrLocationAlreadyExists):
			writeJSONError(w, http.StatusConflict, "an equipment location with this description already exists at this site")
		case errors.Is(err, fireinspectionrows.ErrOKFNumberAlreadyInUse):
			writeJSONError(w, http.StatusConflict, "OKF number is already used by another active extinguisher")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to create fire inspection row")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(row)
}

func (h *fireInspectionRowHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	rowList, err := h.repository.ListByJobID(ctx, jobID)
	if err != nil {
		if errors.Is(err, fireinspectionrows.ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to list fire inspection job rows",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rowList)
}
func (h *fireInspectionRowHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	rowID, ok := pathUUID(w, r, "rowID")
	if !ok {
		return
	}

	var input fireinspectionrows.UpdateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	row, err := h.repository.Update(ctx, jobID, rowID, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionrows.ErrJobNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
		case errors.Is(err, fireinspectionrows.ErrJobNotEditable):
			writeJSONError(
				w,
				http.StatusConflict,
				"fire inspection job is not editable",
			)
		case errors.Is(err, fireinspectionrows.ErrRowNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection row not found")
		case errors.Is(err, fireinspectionrows.ErrRowDoesNotBelongToJob):
			writeJSONError(
				w,
				http.StatusNotFound,
				"fire inspection row not found for this job",
			)
		case errors.Is(err, fireinspectionrows.ErrInvalidRowResult):
			writeJSONError(
				w,
				http.StatusBadRequest,
				"row_result must be one of: ELLENORIZVE, JAVITAS, UJ, HIANYZIK",
			)
		case errors.Is(err, fireinspectionrows.ErrInvalidCapacity):
			writeJSONError(
				w,
				http.StatusBadRequest,
				"capacity_kg must be greater than zero",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to update fire inspection row",
			)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(row)
}

func (h *fireInspectionRowHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	rowID, ok := pathUUID(w, r, "rowID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	if err := h.repository.Delete(ctx, jobID, rowID); err != nil {
		switch {
		case errors.Is(err, fireinspectionrows.ErrJobNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
		case errors.Is(err, fireinspectionrows.ErrJobNotEditable):
			writeJSONError(w, http.StatusConflict, "fire inspection job is not editable")
		case errors.Is(err, fireinspectionrows.ErrRowNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection row not found")
		case errors.Is(err, fireinspectionrows.ErrRowDoesNotBelongToJob):
			writeJSONError(w, http.StatusNotFound, "fire inspection row not found for this job")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to delete fire inspection row")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *fireInspectionRowHandler) Inspect(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	rowID, ok := pathUUID(w, r, "rowID")
	if !ok {
		return
	}

	var input fireinspectionrows.InspectInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	row, err := h.repository.Inspect(ctx, jobID, rowID, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionrows.ErrJobNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
		case errors.Is(err, fireinspectionrows.ErrJobNotEditable):
			writeJSONError(
				w,
				http.StatusConflict,
				"fire inspection job is not editable",
			)
		case errors.Is(err, fireinspectionrows.ErrRowNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection row not found")
		case errors.Is(err, fireinspectionrows.ErrRowHasNoExtinguisher):
			writeJSONError(
				w,
				http.StatusUnprocessableEntity,
				"fire inspection row has no assigned extinguisher",
			)
		case errors.Is(err, fireinspectionrows.ErrExtinguisherNotActive):
			writeJSONError(
				w,
				http.StatusConflict,
				"fire extinguisher is not active at customer",
			)
		case errors.Is(err, fireinspectionrows.ErrExtinguisherWrongLocation):
			writeJSONError(
				w,
				http.StatusConflict,
				"fire extinguisher is not assigned to this equipment location",
			)
		case errors.Is(err, fireinspectionrows.ErrOKFNumberAlreadyInUse):
			writeJSONError(
				w,
				http.StatusConflict,
				"OKF number is already used by another active extinguisher",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to inspect fire extinguisher",
			)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(row)
}
