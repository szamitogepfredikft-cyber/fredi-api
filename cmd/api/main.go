package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")

if databaseURL == "" {
	host := os.Getenv("PGHOST")
	port := os.Getenv("PGPORT")
	database := os.Getenv("PGDATABASE")
	user := os.Getenv("PGUSER")
	password := os.Getenv("PGPASSWORD")

	if host == "" || port == "" || database == "" || user == "" || password == "" {
		log.Fatal("DATABASE_URL or PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD is required")
	}

	databaseURL = "host=" + host +
		" port=" + port +
		" dbname=" + database +
		" user=" + user +
		" password=" + password +
		" sslmode=disable"
}

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("create database pool: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	router := chi.NewRouter()

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := healthResponse{
			Status:   "ok",
			Database: "ok",
		}

		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			response.Status = "degraded"
			response.Database = "unavailable"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on %s", httpAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve HTTP: %v", err)
	}
}
