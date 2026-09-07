package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionduedates"
)

type fireInspectionDueDateHandler struct {
	repository *fireinspectionduedates.Repository
}

func NewFireInspectionDueDateHandler(
	repository *fireinspectionduedates.Repository,
) *fireInspectionDueDateHandler {
	return &fireInspectionDueDateHandler{repository: repository}
}

func (h *fireInspectionDueDateHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	input := fireinspectionduedates.ListInput{
		Query:            r.URL.Query().Get("q"),
		Status:           r.URL.Query().Get("status"),
		DueType:          r.URL.Query().Get("due_type"),
		IncludeCancelled: r.URL.Query().Get("include_cancelled") == "true",
	}

	from, err := parseOptionalDateQuery(r, "from")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "from must be a date in YYYY-MM-DD format")
		return
	}

	to, err := parseOptionalDateQuery(r, "to")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "to must be a date in YYYY-MM-DD format")
		return
	}

	input.From = from
	input.To = to

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	items, err := h.repository.List(ctx, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionduedates.ErrInvalidStatus):
			writeJSONError(w, http.StatusBadRequest, "invalid due date status")
		case errors.Is(err, fireinspectionduedates.ErrInvalidDueType):
			writeJSONError(w, http.StatusBadRequest, "invalid due date type")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to list fire inspection due dates")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fireinspectionduedates.ListResponse{
		Items: items,
		Total: len(items),
	})
}

func (h *fireInspectionDueDateHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input fireinspectionduedates.CreateInput

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

	if input.DueDate == nil || input.DueDate.IsZero() {
		writeJSONError(w, http.StatusBadRequest, "due_date is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	dueDate, err := h.repository.Create(ctx, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionduedates.ErrCustomerOrSiteNotFound):
			writeJSONError(w, http.StatusNotFound, "customer or site not found")
		case errors.Is(err, fireinspectionduedates.ErrInvalidDueType):
			writeJSONError(w, http.StatusBadRequest, "invalid due date type")
		case errors.Is(err, fireinspectionduedates.ErrInvalidCoverageYear):
			writeJSONError(w, http.StatusBadRequest, "coverage_year must be between 2000 and 2100 for annual inspection")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to create fire inspection due date")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/fire-inspection-due-dates/"+dueDate.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dueDate)
}

func (h *fireInspectionDueDateHandler) SyncAnnual(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()

	result, err := h.repository.SyncAnnual(ctx)
	if err != nil {
		log.Printf("annual fire-inspection due-date synchronization failed: %+v", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to synchronize annual fire inspection due dates")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *fireInspectionDueDateHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	dueDateID, ok := pathUUID(w, r, "dueDateID")
	if !ok {
		return
	}

	var input fireinspectionduedates.UpdateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if input.Status != nil {
		value := strings.TrimSpace(*input.Status)
		input.Status = &value
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	dueDate, err := h.repository.Update(ctx, dueDateID, input)
	if err != nil {
		switch {
		case errors.Is(err, fireinspectionduedates.ErrDueDateNotFound):
			writeJSONError(w, http.StatusNotFound, "fire inspection due date not found")
		case errors.Is(err, fireinspectionduedates.ErrInvalidStatus):
			writeJSONError(w, http.StatusBadRequest, "invalid due date status")
		case errors.Is(err, fireinspectionduedates.ErrCancelReasonRequired):
			writeJSONError(w, http.StatusBadRequest, "reschedule_reason is required when status is TOROLVE")
		default:
			writeJSONError(w, http.StatusBadRequest, "failed to update fire inspection due date")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dueDate)
}
