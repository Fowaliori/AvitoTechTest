package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"pr-reviewer/internal/api"
	"pr-reviewer/internal/db"
	"pr-reviewer/internal/service"
)

func main() {
	dbConnStr := os.Getenv("DATABASE_URL")
	if dbConnStr == "" {
		log.Fatal("пустой DATABASE_URL")
	}

	storage, err := db.NewStorage(dbConnStr)
	if err != nil {
		log.Fatal(err)
	}

	svc := service.NewService(storage)
	server := api.NewServer(svc)

	handler := api.Handler(server)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("Server listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
