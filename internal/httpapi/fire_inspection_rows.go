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
