package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"agentrix/backend/internal/api"
	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/matchmaker"
	"agentrix/backend/internal/runner"
	"agentrix/backend/internal/sandbox"
)

func main() {
	log.Println("==================================================")
	log.Println("       AGENTRIX MULTI-AGENT ARENA PLATFORM        ")
	log.Println("             Backend Monolith Service             ")
	log.Println("==================================================")

	for _, arg := range os.Args[1:] {
		if arg == "--migrate-only" || arg == "-migrate-only" {
			dbURL := os.Getenv("DATABASE_URL")
			if dbURL == "" {
				log.Fatalf("[FATAL] DATABASE_URL is required for migrations")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool, err := db.Connect(ctx, dbURL)
			if err != nil {
				log.Fatalf("[FATAL] Could not connect to database: %v", err)
			}
			defer pool.Close()
			if err := applyAllMigrations(ctx, pool); err != nil {
				log.Fatalf("[FATAL] %v", err)
			}
			log.Println("[MIGRATE] All 8 database migrations successfully applied.")
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Invalid configuration: %v", err)
	}
	preflightCtx, preflightCancel := context.WithTimeout(context.Background(), 25*time.Second)
	if err := sandbox.ValidateHost(preflightCtx); err != nil {
		preflightCancel()
		log.Fatalf("[FATAL] Participant sandbox is not available; refusing to start API/queue: %v", err)
	}
	preflightCancel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Connect to PostgreSQL
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to database: %v", err)
	}
	defer pool.Close()

	// 2. Run Database Migrations
	if err := applyAllMigrations(ctx, pool); err != nil {
		log.Fatalf("[FATAL] %v", err)
	}

	// 3. Explicit first-install bootstrap; never create demo accounts or unadmitted bots.
	if err := pool.Bootstrap(ctx, cfg.BootstrapAdminUsername, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
		log.Fatalf("[FATAL] Bootstrap failed: %v", err)
	}

	// 4. Reconcile old immutable artifacts before workers can claim work.
	matchRunner := runner.NewMatchRunner(cfg, pool)
	if removed, err := matchRunner.ReconcileOrphanReplays(ctx, cfg.ReplaysDir); err != nil {
		log.Fatalf("[FATAL] Replay reconciliation failed: %v", err)
	} else if removed > 0 {
		log.Printf("[RUNNER] Removed %d stale unreferenced replay attempt files", removed)
	}

	// 5. Initialize Simulation Runner & Matchmaker
	queueDone := make(chan struct{})
	go func() { defer close(queueDone); matchRunner.RunQueue(ctx) }()
	mm := matchmaker.NewMatchmaker(cfg, pool, matchRunner)
	mm.Start(ctx)
	defer mm.Stop()

	// 6. Initialize API Server
	apiServer := api.NewServer(cfg, pool, matchRunner, mm)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           apiServer.Router(),
		ReadTimeout:       300 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
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
	cancel()
	<-queueDone
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server shutdown failed: %v", err)
	}

	log.Println("[SERVER] Agentrix Backend Monolith cleanly stopped.")
}

func findMigrationsDir() string {
	if custom := os.Getenv("MIGRATIONS_DIR"); custom != "" {
		if _, err := os.Stat(filepath.Join(custom, "000001_init_schema.up.sql")); err == nil {
			return custom
		}
	}
	candidates := []string{
		"backend/migrations",
		"migrations",
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "migrations"),
			filepath.Join(exeDir, "../migrations"),
			filepath.Join(exeDir, "../backend/migrations"),
			filepath.Join(exeDir, "../../backend/migrations"),
			filepath.Join(exeDir, "../../../backend/migrations"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "000001_init_schema.up.sql")); err == nil {
			return c
		}
	}
	return "migrations"
}

func applyAllMigrations(ctx context.Context, pool *db.Pool) error {
	dir := findMigrationsDir()
	files := []string{
		"000001_init_schema.up.sql",
		"000002_match_leases.up.sql",
		"000003_arena_freeze.up.sql",
		"000004_package_integrity.up.sql",
		"000005_tournament_rounds.up.sql",
		"000006_rules_snapshot.up.sql",
		"000007_replay_path_index.up.sql",
		"000008_runtime_provenance.up.sql",
		"000009_contest_phases.up.sql",
	}
	for _, file := range files {
		path := filepath.Join(dir, file)
		if err := pool.RunMigrations(ctx, path); err != nil {
			return fmt.Errorf("migration %s failed: %w", file, err)
		}
	}
	return nil
}
