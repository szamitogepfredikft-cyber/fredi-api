package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hajdurenato/fredi-api/internal/extinguisherassignments"
)

type extinguisherAssignmentHandler struct {
	repository *extinguisherassignments.Repository
}

type createExtinguisherAssignmentRequest struct {
	EquipmentLocationID string  `json:"equipment_location_id"`
	AssignmentReason    string  `json:"assignment_reason"`
	Notes               *string `json:"notes"`
}

func NewExtinguisherAssignmentHandler(
	repository *extinguisherassignments.Repository,
) *extinguisherAssignmentHandler {
	return &extinguisherAssignmentHandler{
		repository: repository,
	}
}

func (h *extinguisherAssignmentHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	extinguisherID, ok := pathUUID(w, r, "extinguisherID")
	if !ok {
		return
	}

	var request createExtinguisherAssignmentRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	equipmentLocationID, err := uuid.Parse(strings.TrimSpace(request.EquipmentLocationID))
	if err != nil || equipmentLocationID == uuid.Nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment_location_id")
		return
	}

	assignmentReason := strings.TrimSpace(request.AssignmentReason)
	if assignmentReason == "" {
		writeJSONError(w, http.StatusBadRequest, "assignment_reason is required")
		return
	}

	if !extinguisherassignments.IsValidAssignmentReason(assignmentReason) {
		writeJSONError(w, http.StatusBadRequest, "invalid assignment_reason")
		return
	}

	input := extinguisherassignments.CreateInput{
		EquipmentLocationID: equipmentLocationID,
		AssignmentReason:    assignmentReason,
		Notes:               request.Notes,
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	assignment, err := h.repository.Create(ctx, extinguisherID, input)
	if err != nil {
		switch {
		case errors.Is(err, extinguisherassignments.ErrExtinguisherNotFound):
			writeJSONError(w, http.StatusNotFound, "extinguisher not found")
		case errors.Is(err, extinguisherassignments.ErrEquipmentLocationNotFound):
			writeJSONError(w, http.StatusNotFound, "equipment location not found")
		case errors.Is(err, extinguisherassignments.ErrExtinguisherNotIssuable):
			writeJSONError(w, http.StatusConflict, "extinguisher is not issuable")
		case errors.Is(err, extinguisherassignments.ErrExtinguisherAlreadyAssigned):
			writeJSONError(w, http.StatusConflict, "extinguisher already has an active assignment")
		case errors.Is(err, extinguisherassignments.ErrEquipmentLocationAlreadyUsed):
			writeJSONError(w, http.StatusConflict, "equipment location already has an active assignment")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to create extinguisher assignment")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(assignment)
}

func (h *extinguisherAssignmentHandler) ListByExtinguisher(
	w http.ResponseWriter,
	r *http.Request,
) {
	extinguisherID, ok := pathUUID(w, r, "extinguisherID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	assignments, err := h.repository.ListByExtinguisher(ctx, extinguisherID)
	if err != nil {
		if errors.Is(err, extinguisherassignments.ErrExtinguisherNotFound) {
			writeJSONError(w, http.StatusNotFound, "extinguisher not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to list extinguisher assignments")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assignments)
}

type unassignExtinguisherRequest struct {
	UnassignmentReason string  `json:"unassignment_reason"`
	Notes              *string `json:"notes"`
}

func (h *extinguisherAssignmentHandler) Unassign(
	w http.ResponseWriter,
	r *http.Request,
) {
	extinguisherID, ok := pathUUID(w, r, "extinguisherID")
	if !ok {
		return
	}

	var request unassignExtinguisherRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	unassignmentReason := strings.TrimSpace(request.UnassignmentReason)
	if unassignmentReason == "" {
		writeJSONError(w, http.StatusBadRequest, "unassignment_reason is required")
		return
	}

	if !extinguisherassignments.IsValidUnassignmentReason(unassignmentReason) {
		writeJSONError(w, http.StatusBadRequest, "invalid unassignment_reason")
		return
	}

	input := extinguisherassignments.UnassignInput{
		UnassignmentReason: unassignmentReason,
		Notes:              request.Notes,
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	assignment, err := h.repository.Unassign(ctx, extinguisherID, input)
	if err != nil {
		switch {
		case errors.Is(err, extinguisherassignments.ErrExtinguisherNotFound):
			writeJSONError(w, http.StatusNotFound, "extinguisher not found")
		case errors.Is(err, extinguisherassignments.ErrActiveAssignmentNotFound):
			writeJSONError(w, http.StatusConflict, "extinguisher has no active assignment")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to unassign extinguisher")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assignment)
}
