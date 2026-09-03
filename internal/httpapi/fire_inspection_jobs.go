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

func (h *fireInspectionJobHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	input := fireinspectionjobs.ListInput{
		View:   r.URL.Query().Get("view"),
		Status: r.URL.Query().Get("status"),
		Query:  r.URL.Query().Get("q"),
	}

	from, err := parseOptionalDateQuery(r, "from")
	if err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"from must be a date in YYYY-MM-DD format",
		)
		return
	}

	to, err := parseOptionalDateQuery(r, "to")
	if err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"to must be a date in YYYY-MM-DD format",
		)
		return
	}

	input.From = from
	input.To = to

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	items, err := h.repository.List(ctx, input)
	if err != nil {
		if err.Error() == "invalid fire inspection job list view" {
			writeJSONError(
				w,
				http.StatusBadRequest,
				"view must be one of: open, closed, archived",
			)
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to list fire inspection jobs",
		)
		return
	}

	response := fireinspectionjobs.ListResponse{
		Items: items,
		Total: len(items),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
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
		switch {
		case errors.Is(err, fireinspectionjobs.ErrCustomerOrSiteNotFound):
			writeJSONError(w, http.StatusNotFound, "customer or site not found")
		case errors.Is(err, fireinspectionjobs.ErrInspectorNotSelectable):
			writeJSONError(
				w,
				http.StatusConflict,
				"the selected inspector must be active and have exactly one active certificate",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to create fire inspection job",
			)
		}
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

func (h *fireInspectionJobHandler) Complete(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	job, err := h.repository.Complete(ctx, jobID)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionjobs.ErrJobNotFound):
			writeJSONError(
				w,
				http.StatusNotFound,
				"fire inspection job not found",
			)
		case errors.Is(err, fireinspectionjobs.ErrJobNotCompletable):
			writeJSONError(
				w,
				http.StatusConflict,
				"fire inspection job is not completable",
			)
		case errors.Is(err, fireinspectionjobs.ErrUncheckedRows):
			writeJSONError(
				w,
				http.StatusConflict,
				"all inspection rows must be processed before completion",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to complete fire inspection job",
			)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func (h *fireInspectionJobHandler) Reopen(
	w http.ResponseWriter,
	r *http.Request,
) {
	jobID, ok := pathUUID(w, r, "jobID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	job, err := h.repository.Reopen(ctx, jobID)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionjobs.ErrJobNotFound):
			writeJSONError(
				w,
				http.StatusNotFound,
				"fire inspection job not found",
			)
		case errors.Is(err, fireinspectionjobs.ErrJobNotReopenable):
			writeJSONError(
				w,
				http.StatusConflict,
				"only completed fire inspection jobs can be reopened",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to reopen fire inspection job",
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
func parseOptionalDateQuery(
	r *http.Request,
	name string,
) (*fireinspectionjobs.Date, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}

	return &fireinspectionjobs.Date{
		Time: parsed,
	}, nil
}
