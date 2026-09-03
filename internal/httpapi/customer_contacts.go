package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hajdurenato/fredi-api/internal/customercontacts"
)

type customerContactHandler struct {
	repository *customercontacts.Repository
}

func NewCustomerContactHandler(
	repository *customercontacts.Repository,
) *customerContactHandler {
	return &customerContactHandler{repository: repository}
}

func (h *customerContactHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	customerID, ok := pathUUID(w, r, "customerID")
	if !ok {
		return
	}

	var input customercontacts.CreateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.Name) == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	contact, err := h.repository.Create(ctx, customerID, input)
	if err != nil {
		switch {
		case errors.Is(err, customercontacts.ErrCustomerNotFound):
			writeJSONError(w, http.StatusNotFound, "customer not found")
		case errors.Is(err, customercontacts.ErrSiteNotFound):
			writeJSONError(w, http.StatusNotFound, "site not found")
		case errors.Is(err, customercontacts.ErrSiteDoesNotBelong):
			writeJSONError(
				w,
				http.StatusBadRequest,
				"site does not belong to customer",
			)
		case errors.Is(err, customercontacts.ErrPrimaryContactExists):
			writeJSONError(
				w,
				http.StatusConflict,
				"an active primary contact already exists for this customer or site",
			)
		case errors.Is(err, customercontacts.ErrInvalidContactMethod):
			writeJSONError(
				w,
				http.StatusBadRequest,
				"preferred_contact_method must be one of: PHONE, EMAIL, PERSONAL, OTHER",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to create customer contact",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(
		"Location",
		"/api/v1/customers/"+customerID.String()+"/contacts/"+contact.ID.String(),
	)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(contact)
}

func (h *customerContactHandler) ListByCustomer(
	w http.ResponseWriter,
	r *http.Request,
) {
	customerID, ok := pathUUID(w, r, "customerID")
	if !ok {
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	contacts, err := h.repository.ListByCustomer(ctx, customerID)
	if err != nil {
		if errors.Is(err, customercontacts.ErrCustomerNotFound) {
			writeJSONError(w, http.StatusNotFound, "customer not found")
			return
		}

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to list customer contacts",
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contacts)
}

func (h *customerContactHandler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	customerID, ok := pathUUID(w, r, "customerID")
	if !ok {
		return
	}

	contactID, ok := pathUUID(w, r, "contactID")
	if !ok {
		return
	}

	var input customercontacts.UpdateInput

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if strings.TrimSpace(input.Name) == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	contact, err := h.repository.Update(ctx, customerID, contactID, input)
	if err != nil {
		switch {
		case errors.Is(err, customercontacts.ErrCustomerNotFound):
			writeJSONError(w, http.StatusNotFound, "customer not found")
		case errors.Is(err, customercontacts.ErrContactNotFound):
			writeJSONError(w, http.StatusNotFound, "customer contact not found")
		case errors.Is(err, customercontacts.ErrSiteNotFound):
			writeJSONError(w, http.StatusNotFound, "site not found")
		case errors.Is(err, customercontacts.ErrSiteDoesNotBelong):
			writeJSONError(
				w,
				http.StatusBadRequest,
				"site does not belong to customer",
			)
		case errors.Is(err, customercontacts.ErrPrimaryContactExists):
			writeJSONError(
				w,
				http.StatusConflict,
				"an active primary contact already exists for this customer or site",
			)
		case errors.Is(err, customercontacts.ErrInvalidContactMethod):
			writeJSONError(
				w,
				http.StatusBadRequest,
				"preferred_contact_method must be one of: PHONE, EMAIL, PERSONAL, OTHER",
			)
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"failed to update customer contact",
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contact)
}
