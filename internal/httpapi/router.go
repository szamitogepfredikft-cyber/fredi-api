package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hajdurenato/fredi-api/internal/customers"
	"github.com/hajdurenato/fredi-api/internal/equipmentlocations"
	"github.com/hajdurenato/fredi-api/internal/sites"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(db *pgxpool.Pool) http.Handler {
	router := chi.NewRouter()

	router.Get("/health", HealthHandler(db))

	customerRepository := customers.NewRepository(db)
	customerHandler := NewCustomerHandler(customerRepository)

	siteRepository := sites.NewRepository(db)
	siteHandler := NewSiteHandler(siteRepository)

	equipmentLocationRepository := equipmentlocations.NewRepository(db)
	equipmentLocationHandler := NewEquipmentLocationHandler(equipmentLocationRepository)

	router.Route("/api/v1", func(router chi.Router) {
		router.Route("/customers", func(router chi.Router) {
			router.Post("/", customerHandler.Create)
			router.Get("/", customerHandler.List)

			router.Route("/{customerID}/sites", func(router chi.Router) {
				router.Post("/", siteHandler.Create)
				router.Get("/", siteHandler.ListByCustomer)
			})
		})

		router.Route("/sites/{siteID}/equipment-locations", func(router chi.Router) {
			router.Post("/", equipmentLocationHandler.Create)
			router.Get("/", equipmentLocationHandler.ListBySite)
		})

		router.Get("/sites/{siteID}", siteHandler.GetByID)
		router.Get("/equipment-locations/{locationID}", equipmentLocationHandler.GetByID)
	})

	return router
}
