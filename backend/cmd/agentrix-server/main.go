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

	"agentrix/backend/internal/api"
	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/matchmaker"
	"agentrix/backend/internal/runner"
)

func main() {
	log.Println("==================================================")
	log.Println("       AGENTRIX MULTI-AGENT ARENA PLATFORM        ")
	log.Println("             Backend Monolith Service             ")
	log.Println("==================================================")

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Connect to PostgreSQL
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to database: %v", err)
	}
	defer pool.Close()

	// 2. Run Database Migrations
	migrationPath := "backend/migrations/000001_init_schema.up.sql"
	if _, err := os.Stat(migrationPath); err != nil {
		migrationPath = "migrations/000001_init_schema.up.sql"
	}
	if err := pool.RunMigrations(ctx, migrationPath); err != nil {
		log.Printf("[WARN] Running migration returned: %v (continuing)", err)
	}

	// 3. Auto-seed initial data if empty
	if err := pool.AutoSeed(ctx); err != nil {
		log.Printf("[WARN] AutoSeed returned: %v (continuing)", err)
	}

	// 4. Initialize Simulation Runner & Matchmaker
	matchRunner := runner.NewMatchRunner(cfg, pool)
	mm := matchmaker.NewMatchmaker(cfg, pool, matchRunner)
	mm.Start(ctx)
	defer mm.Stop()

	// 5. Initialize API Server
	apiServer := api.NewServer(cfg, pool, matchRunner, mm)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      apiServer.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("[SERVER] HTTP REST API listening on http://0.0.0.0:%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[SERVER] Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server shutdown failed: %v", err)
	}

	log.Println("[SERVER] Agentrix Backend Monolith cleanly stopped.")
}
