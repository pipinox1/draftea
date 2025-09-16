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

	"github.com/draftea/payment-system/wallet-service/config"
	"github.com/draftea/payment-system/wallet-service/handlers"
	"github.com/draftea/payment-system/shared/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.ReadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting %s in %s environment on port %s\n", cfg.ServiceName, cfg.Env, cfg.Port)

	// Initialize dependencies
	ctx := context.Background()
	deps, err := config.BuildDependencies(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to build dependencies: %v", err)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			log.Printf("Error closing dependencies: %v", err)
		}
	}()

	// Start event subscriber
	go func() {
		ctx := context.Background()
		if err := deps.EventSubscriber.Subscribe(ctx, "", deps.WalletEventHandlers); err != nil {
			log.Printf("Error in event subscriber: %v", err)
		}
	}()

	// Setup HTTP router
	router := setupRouter(cfg, deps)

	// Setup and start HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Printf("Shutting down %s...\n", cfg.ServiceName)

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	fmt.Printf("%s stopped\n", cfg.ServiceName)
}

func setupRouter(cfg *config.Config, deps *config.Dependencies) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// Telemetry middleware (inject telemetry into context)
	if deps.Telemetry != nil {
		r.Use(telemetry.Middleware(deps.Telemetry))
	}

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint for Prometheus
	r.Handle("/metrics", handlers.NewMetricsHandler())

	// Register wallet routes
	deps.WalletHandlers.RegisterRoutes(r)

	return r
}