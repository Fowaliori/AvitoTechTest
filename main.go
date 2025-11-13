package main

import (
	"draft/internal/api"
	"draft/internal/db"
	"draft/internal/service"
	"log"
	"net/http"
)

func main() {
	storage, err := db.NewStorage("postgres://postgres:123@localhost:5433/avitotech?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	svc := service.NewService(storage)
	server := api.NewServer(svc)

	handler := api.Handler(server)
	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
