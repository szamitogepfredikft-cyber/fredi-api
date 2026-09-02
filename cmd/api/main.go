package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hajdurenato/fredi-api/internal/database"
	"github.com/hajdurenato/fredi-api/internal/httpapi"
)

func main() {
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPool(ctx)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           httpapi.NewRouter(db),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on %s", httpAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve HTTP: %v", err)
	}
}
