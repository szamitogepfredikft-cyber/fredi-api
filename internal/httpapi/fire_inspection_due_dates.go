package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
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
		Query:   r.URL.Query().Get("q"),
		Status:  r.URL.Query().Get("status"),
		DueType: r.URL.Query().Get("due_type"),
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
		switch {
		case errors.Is(err, fireinspectionduedates.ErrInvalidStatus):
			writeJSONError(w, http.StatusBadRequest, "invalid due date status")
		case errors.Is(err, fireinspectionduedates.ErrInvalidDueType):
			writeJSONError(w, http.StatusBadRequest, "invalid due date type")
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to list fire inspection due dates",
			)
		}
		return
	}

	response := fireinspectionduedates.ListResponse{
		Items: items,
		Total: len(items),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
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
			writeJSONError(
				w,
				http.StatusBadRequest,
				"coverage_year must be between 2000 and 2100 for annual inspection",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to create fire inspection due date",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(
		"Location",
		"/api/v1/fire-inspection-due-dates/"+dueDate.ID.String(),
	)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dueDate)
}
