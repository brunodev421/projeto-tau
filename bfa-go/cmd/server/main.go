package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/itau-ai-assistant/bfa-go/internal/api"
	"github.com/itau-ai-assistant/bfa-go/internal/cache"
	"github.com/itau-ai-assistant/bfa-go/internal/clients"
	"github.com/itau-ai-assistant/bfa-go/internal/config"
	"github.com/itau-ai-assistant/bfa-go/internal/handlers"
	"github.com/itau-ai-assistant/bfa-go/internal/models"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"github.com/itau-ai-assistant/bfa-go/internal/resilience"
	"github.com/itau-ai-assistant/bfa-go/internal/service"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := observability.NewLogger(cfg.ServiceName)
	shutdownTracer, err := observability.InitTracer(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	if err != nil {
		log.Fatalf("init tracer: %v", err)
	}
	defer func() {
		_ = shutdownTracer(context.Background())
	}()

	registry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(registry)
	retryPolicy := resilience.NewRetryPolicy(cfg.RetryMaxAttempts, metrics)
	bulkhead := resilience.NewBulkhead(cfg.BulkheadLimit, metrics)

	profileClient := clients.NewProfileClient(
		cfg.ProfileAPIURL,
		cfg.DownstreamTimeout,
		retryPolicy,
		resilience.NewCircuitBreaker("profile_api", cfg.CircuitBreakerFailRatio, cfg.CircuitBreakerMinRequests, cfg.CircuitBreakerOpenTimeout, metrics),
		bulkhead,
		metrics,
	)
	transactionsClient := clients.NewTransactionsClient(
		cfg.TransactionsAPIURL,
		cfg.DownstreamTimeout,
		retryPolicy,
		resilience.NewCircuitBreaker("transactions_api", cfg.CircuitBreakerFailRatio, cfg.CircuitBreakerMinRequests, cfg.CircuitBreakerOpenTimeout, metrics),
		bulkhead,
		metrics,
	)
	agentClient := clients.NewAgentClient(
		cfg.AgentServiceURL,
		cfg.DownstreamTimeout,
		retryPolicy,
		resilience.NewCircuitBreaker("agent_service", cfg.CircuitBreakerFailRatio, cfg.CircuitBreakerMinRequests, cfg.CircuitBreakerOpenTimeout, metrics),
		bulkhead,
		metrics,
	)

	assistantService := service.NewAssistantService(
		profileClient,
		transactionsClient,
		agentClient,
		cache.New[models.Profile](),
		cfg.ProfileCacheTTL,
		logger,
		metrics,
	)

	handler := handlers.NewAssistantHandler(assistantService)
	router := api.NewRouter(logger, metrics, registry, handler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting bfa server", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
	fmt.Println("bfa shutdown complete")
}
