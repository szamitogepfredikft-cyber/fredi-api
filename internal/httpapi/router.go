package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hajdurenato/fredi-api/internal/customers"
	"github.com/hajdurenato/fredi-api/internal/equipmentlocations"
	"github.com/hajdurenato/fredi-api/internal/extinguisherassignments"
	"github.com/hajdurenato/fredi-api/internal/extinguishers"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionjobs"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionrows"
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

	extinguisherRepository := extinguishers.NewRepository(db)
	extinguisherHandler := NewExtinguisherHandler(extinguisherRepository)
	extinguisherAssignmentRepository := extinguisherassignments.NewRepository(db)
	extinguisherAssignmentHandler := NewExtinguisherAssignmentHandler(
		extinguisherAssignmentRepository,
	)

	fireInspectionJobRepository := fireinspectionjobs.NewRepository(db)
	fireInspectionJobHandler := NewFireInspectionJobHandler(
		fireInspectionJobRepository,
	)

	fireInspectionRowRepository := fireinspectionrows.NewRepository(db)
	fireInspectionRowHandler := NewFireInspectionRowHandler(
		fireInspectionRowRepository,
	)

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

		router.Route("/extinguishers", func(router chi.Router) {
			router.Post("/", extinguisherHandler.Create)
			router.Get("/", extinguisherHandler.List)
			router.Post(
				"/{extinguisherID}/assignments",
				extinguisherAssignmentHandler.Create,
			)
			router.Post(
				"/{extinguisherID}/unassignments",
				extinguisherAssignmentHandler.Unassign,
			)
			router.Get(
				"/{extinguisherID}/assignments",
				extinguisherAssignmentHandler.ListByExtinguisher,
			)
			router.Get("/{extinguisherID}", extinguisherHandler.GetByID)
		})

		router.Route("/fire-inspection-jobs", func(router chi.Router) {
			router.Post("/", fireInspectionJobHandler.Create)
			router.Get("/", fireInspectionJobHandler.List)

			router.Get("/{jobID}", fireInspectionJobHandler.GetByID)
			router.Patch("/{jobID}", fireInspectionJobHandler.Update)
			router.Post(
				"/{jobID}:complete",
				fireInspectionJobHandler.Complete,
			)
			router.Post(
				"/{jobID}:reopen",
				fireInspectionJobHandler.Reopen,
			)
			router.Post(
				"/{jobID}/rows:initialize",
				fireInspectionRowHandler.Initialize,
			)

			router.Get(
				"/{jobID}/rows",
				fireInspectionRowHandler.List,
			)

			router.Post(
				"/{jobID}/rows/{rowID}:inspect",
				fireInspectionRowHandler.Inspect,
			)

			router.Patch(
				"/{jobID}/rows/{rowID}",
				fireInspectionRowHandler.Update,
			)
		})

		router.Get("/sites/{siteID}", siteHandler.GetByID)
		router.Get("/equipment-locations/{locationID}", equipmentLocationHandler.GetByID)
		router.Get(
			"/equipment-locations/{locationID}/assignment",
			extinguisherAssignmentHandler.GetActiveByEquipmentLocation,
		)
	})

	return router
}
