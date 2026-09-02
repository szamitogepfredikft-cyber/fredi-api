package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hajdurenato/fredi-api/internal/customers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(db *pgxpool.Pool) http.Handler {
	router := chi.NewRouter()

	router.Get("/health", HealthHandler(db))

	customerRepository := customers.NewRepository(db)
	customerHandler := NewCustomerHandler(customerRepository)

	router.Route("/api/v1", func(router chi.Router) {
		router.Route("/customers", func(router chi.Router) {
			router.Post("/", customerHandler.Create)
			router.Get("/", customerHandler.List)
		})
	})

	return router
}
