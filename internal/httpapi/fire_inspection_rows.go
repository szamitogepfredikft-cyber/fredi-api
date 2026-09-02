package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
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
				"row_result must be one of: ELLENORIZVE, JAVITAS, HIANYZIK",
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
