package main

import (
	"draft/internal/api"
	"draft/internal/db"
	"draft/internal/service"
	"log"
	"net/http"
	"os"
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

	log.Printf("Server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
