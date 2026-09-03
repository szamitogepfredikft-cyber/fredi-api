package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionjobs"
)

type fireInspectionJobHandler struct {
	repository *fireinspectionjobs.Repository
}

func NewFireInspectionJobHandler(
	repository *fireinspectionjobs.Repository,
) *fireInspectionJobHandler {
	return &fireInspectionJobHandler{repository: repository}
}

func (h *fireInspectionJobHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input fireinspectionjobs.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if input.CustomerID == uuid.Nil {
		writeJSONError(w, http.StatusBadRequest, "customer_id is required")
		return
	}

	if input.SiteID == uuid.Nil {
		writeJSONError(w, http.StatusBadRequest, "site_id is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	job, err := h.repository.Create(ctx, input)
	if err != nil {
		if errors.Is(err, fireinspectionjobs.ErrCustomerOrSiteNotFound) {
			writeJSONError(w, http.StatusNotFound, "customer or site not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to create fire inspection job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(
		"Location",
		"/api/v1/fire-inspection-jobs/"+job.ID.String(),
	)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(job)
}

func (h *fireInspectionJobHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	var input fireinspectionjobs.UpdateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	job, err := h.repository.Update(ctx, jobID, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionjobs.ErrJobNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
		case errors.Is(err, fireinspectionjobs.ErrJobNotEditable):
			writeJSONError(
				w,
				http.StatusConflict,
				"fire inspection job is not editable",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to update fire inspection job",
			)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func (h *fireInspectionJobHandler) GetByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	job, err := h.repository.GetByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, fireinspectionjobs.ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "fire inspection job not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to get fire inspection job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}
