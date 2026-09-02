package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hajdurenato/fredi-api/internal/customers"
)

type customerHandler struct {
	repository *customers.Repository
}

func NewCustomerHandler(repository *customers.Repository) *customerHandler {
	return &customerHandler{repository: repository}
}

func (h *customerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input customers.CreateInput

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

	customer, err := h.repository.Create(ctx, input)
	if err != nil {
		if errors.Is(err, customers.ErrTaxNumberAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "an active customer with this tax number already exists")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to create customer")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/customers/"+customer.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(customer)
}

func (h *customerHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	customerList, err := h.repository.List(ctx)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list customers")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(customerList)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
