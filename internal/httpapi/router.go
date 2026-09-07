package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hajdurenato/fredi-api/internal/customercontacts"
	"github.com/hajdurenato/fredi-api/internal/customers"
	"github.com/hajdurenato/fredi-api/internal/equipmentlocations"
	"github.com/hajdurenato/fredi-api/internal/extinguisherassignments"
	"github.com/hajdurenato/fredi-api/internal/extinguishers"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionduedates"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionjobs"
	"github.com/hajdurenato/fredi-api/internal/fireinspectionrows"
	"github.com/hajdurenato/fredi-api/internal/inspectors"
	"github.com/hajdurenato/fredi-api/internal/sites"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(db *pgxpool.Pool) http.Handler {
	router := chi.NewRouter()

	router.Get("/health", HealthHandler(db))
	customerRepository := customers.NewRepository(db)
	customerHandler := NewCustomerHandler(customerRepository)
	customerContactRepository := customercontacts.NewRepository(db)
	customerContactHandler := NewCustomerContactHandler(
		customerContactRepository,
	)

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
	fireInspectionDueDateRepository := fireinspectionduedates.NewRepository(db)
	fireInspectionDueDateHandler := NewFireInspectionDueDateHandler(
		fireInspectionDueDateRepository,
	)

	fireInspectionRowRepository := fireinspectionrows.NewRepository(db)
	fireInspectionRowHandler := NewFireInspectionRowHandler(
		fireInspectionRowRepository,
	)

	inspectorRepository := inspectors.NewRepository(db)
	inspectorHandler := NewInspectorHandler(inspectorRepository)

	router.Route("/api/v1", func(router chi.Router) {
		router.Route("/customers", func(router chi.Router) {
			router.Post("/", customerHandler.Create)
			router.Get("/", customerHandler.List)
			router.Get("/{customerID}", customerHandler.GetByID)
			router.Patch("/{customerID}", customerHandler.Update)

			router.Delete("/{customerID}", customerHandler.Archive)
			router.Route("/{customerID}/contacts", func(router chi.Router) {
				router.Post("/", customerContactHandler.Create)
				router.Get("/", customerContactHandler.ListByCustomer)
				router.Patch("/{contactID}", customerContactHandler.Update)
			})

			router.Route("/{customerID}/sites", func(router chi.Router) {
				router.Post("/", siteHandler.Create)
				router.Get("/", siteHandler.ListByCustomer)
			})
		})

		router.Patch("/sites/{siteID}", siteHandler.Update)

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

		router.Route("/inspectors", func(router chi.Router) {
			router.Get("/", inspectorHandler.List)
			router.Post("/", inspectorHandler.Create)
			router.Get("/{inspectorID}", inspectorHandler.GetByID)
			router.Patch("/{inspectorID}", inspectorHandler.Update)
			router.Post("/{inspectorID}:archive", inspectorHandler.Archive)
			router.Post("/{inspectorID}:restore", inspectorHandler.Restore)

			router.Post(
				"/{inspectorID}/certificates",
				inspectorHandler.CreateCertificate,
			)
			router.Patch(
				"/{inspectorID}/certificates/{certificateID}",
				inspectorHandler.UpdateCertificate,
			)
			router.Post(
				"/{inspectorID}/certificates/{certificateID}:archive",
				inspectorHandler.ArchiveCertificate,
			)
			router.Post(
				"/{inspectorID}/certificates/{certificateID}:restore",
				inspectorHandler.RestoreCertificate,
			)
		})

		router.Route("/fire-inspection-due-dates", func(router chi.Router) {
			router.Get("/", fireInspectionDueDateHandler.List)
			router.Post("/", fireInspectionDueDateHandler.Create)
			router.Post("/:sync-annual", fireInspectionDueDateHandler.SyncAnnual)
			router.Patch("/{dueDateID}", fireInspectionDueDateHandler.Update)
		})

		router.Route("/fire-inspection-jobs", func(router chi.Router) {
			router.Post("/", fireInspectionJobHandler.Create)
			router.Get("/", fireInspectionJobHandler.List)

			router.Get("/{jobID}", fireInspectionJobHandler.GetByID)
			router.Patch("/{jobID}", fireInspectionJobHandler.Update)
			router.Delete("/{jobID}", fireInspectionJobHandler.DeleteDraft)
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
				"/{jobID}/rows",
				fireInspectionRowHandler.Create,
			)
			router.Post(
				"/{jobID}/rows/{rowID}:inspect",
				fireInspectionRowHandler.Inspect,
			)

			router.Patch(
				"/{jobID}/rows/{rowID}",
				fireInspectionRowHandler.Update,
			)
			router.Delete(
				"/{jobID}/rows/{rowID}",
				fireInspectionRowHandler.Delete,
			)
		})

		router.Get("/sites/{siteID}", siteHandler.GetByID)
		router.Get("/equipment-locations/{locationID}", equipmentLocationHandler.GetByID)
		router.Patch("/equipment-locations/{locationID}", equipmentLocationHandler.Update)
		router.Get(
			"/equipment-locations/{locationID}/assignment",
			extinguisherAssignmentHandler.GetActiveByEquipmentLocation,
		)
	})

	return router
}
