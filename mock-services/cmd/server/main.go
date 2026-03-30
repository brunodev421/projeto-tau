package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/itau-ai-assistant/mock-services/internal/handlers"
	"github.com/itau-ai-assistant/mock-services/internal/service"
)

func main() {
	port := os.Getenv("MOCK_PORT")
	if port == "" {
		port = "8081"
	}

	repository := service.NewRepository()
	handler := handlers.New(repository)

	router := chi.NewRouter()
	router.Get("/healthz", handlers.Healthz)
	router.Get("/readyz", handlers.Readyz)
	router.Get("/metrics", handlers.Metrics)
	router.Get("/profile/{customerId}", handlers.BuildRoute(handler.Profile))
	router.Get("/transactions/{customerId}", handlers.BuildRoute(handler.Transactions))

	log.Printf("mock services listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}
