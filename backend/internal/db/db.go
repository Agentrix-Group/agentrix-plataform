package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Pool struct {
	*pgxpool.Pool
}

func Connect(ctx context.Context, connString string) (*Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("error parsing database URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	log.Println("[DB] Connected to PostgreSQL successfully")
	return &Pool{Pool: pool}, nil
}

func (p *Pool) RunMigrations(ctx context.Context, migrationPath string) error {
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
	}

	_, err = p.Exec(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed executing migration %s: %w", migrationPath, err)
	}

	log.Printf("[DB] Applied migration %s successfully", migrationPath)
	return nil
}

func (p *Pool) AutoSeed(ctx context.Context) error {
	var count int
	err := p.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Println("[DB] Database already seeded, skipping auto-seed")
		return nil
	}

	log.Println("[DB] Seeding initial database with default admin, teams, arenas, and bots...")

	adminHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	playerHash, err := bcrypt.GenerateFromPassword([]byte("player123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Insert admin user
	var adminID int
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, 'admin')
		RETURNING id
	`, "admin", "admin@agentrix.internal", string(adminHash)).Scan(&adminID)
	if err != nil {
		return err
	}

	// 2. Insert Player Users & Teams
	teamSpecs := []struct {
		Username    string
		Email       string
		TeamName    string
		Affiliation string
		AgentName   string
		Entrypoint  string
	}{
		{"alpha_lead", "alpha@agentrix.internal", "Alpha Robotics", "MIT AI Lab", "Heuristic-Alpha", "python3 agentrix/bots/heuristic_bot/agent.py"},
		{"beta_lead", "beta@agentrix.internal", "Beta Cybernetics", "Stanford Agents", "ONNX-Hunter", "python3 agentrix/bots/onnx_bot/agent.py"},
		{"gamma_lead", "gamma@agentrix.internal", "Gamma Neural", "Oxford DeepLab", "CPP-Vanguard", "agentrix/bots/cpp_bot/bot"},
		{"delta_lead", "delta@agentrix.internal", "Delta Systems", "ETH Zurich", "Random-Wanderer", "python3 agentrix/bots/heuristic_bot/agent.py"},
		{"omega_lead", "omega@agentrix.internal", "Omega Autonomous", "Tokyo Tech", "Sniper-Apex", "python3 agentrix/bots/onnx_bot/agent.py"},
	}

	// 3. Insert Default Arena
	var arenaID int
	err = tx.QueryRow(ctx, `
		INSERT INTO arenas (slug, name, description, game_type, max_players, max_ticks, config_json, is_active)
		VALUES ($1, $2, $3, $4, 5, 1000, '{"grid_width": 1000, "grid_height": 1000, "zone_shrink_interval": 100}'::jsonb, true)
		RETURNING id
	`, "battle-royale-5p", "Battle Royale 5-Player Arena", "Official competitive 2D arena where 5 autonomous bots navigate, gather items, engage in combat, and survive the closing zone.", "battle_royale_5p").Scan(&arenaID)
	if err != nil {
		return err
	}

	for _, spec := range teamSpecs {
		var userID int
		err = tx.QueryRow(ctx, `
			INSERT INTO users (username, email, password_hash, role)
			VALUES ($1, $2, $3, 'player')
			RETURNING id
		`, spec.Username, spec.Email, string(playerHash)).Scan(&userID)
		if err != nil {
			return err
		}

		var teamID int
		err = tx.QueryRow(ctx, `
			INSERT INTO teams (name, affiliation, owner_id)
			VALUES ($1, $2, $3)
			RETURNING id
		`, spec.TeamName, spec.Affiliation, userID).Scan(&teamID)
		if err != nil {
			return err
		}

		artifactPath := fmt.Sprintf("/var/agentrix/bots/%s", spec.AgentName)
		var agentID int
		err = tx.QueryRow(ctx, `
			INSERT INTO agent_versions (team_id, arena_id, name, version, runtime, entrypoint, artifact_path, sha256, status)
			VALUES ($1, $2, $3, 1, 'python-standard', $4, $5, 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', 'active')
			RETURNING id
		`, teamID, arenaID, spec.AgentName, spec.Entrypoint, artifactPath).Scan(&agentID)
		if err != nil {
			return err
		}

		// Initial ladder entry
		_, err = tx.Exec(ctx, `
			INSERT INTO ladder_entries (arena_id, agent_version_id, rating_mu, rating_sigma, display_rating, matches_played, wins, kills, survival_ticks_total)
			VALUES ($1, $2, 1500.0, 350.0, 1500, 0, 0, 0, 0)
		`, arenaID, agentID)
		if err != nil {
			return err
		}
	}

	// Insert initial audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (user_id, action, target_type, target_id, ip_address, details_json)
		VALUES ($1, 'SYSTEM_INIT', 'SYSTEM', 'INITIAL_SEED', '127.0.0.1', '{"message": "Auto-seeded default arena, teams, and sample bots"}'::jsonb)
	`, adminID)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	log.Println("[DB] Initial auto-seeding completed successfully!")
	return nil
}
