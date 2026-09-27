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

// Bootstrap creates an administrator and an empty arena only when explicitly requested.
// The transaction lock serializes concurrent first starts. Existing administrators
// are never overwritten and sample agents must enter through normal ZIP admission.
func (p *Pool) Bootstrap(ctx context.Context, username, email, password string) error {
	if username == "" && email == "" && password == "" {
		return nil
	}
	if username == "" || len(username) > 64 || email == "" || len(email) > 255 || len(password) < 16 || len(password) > 72 {
		return fmt.Errorf("invalid bootstrap credentials")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1096307272, 1)"); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE role='admin')").Scan(&exists); err != nil {
		return err
	}
	if exists {
		return tx.Commit(ctx)
	}
	var id int
	if err := tx.QueryRow(ctx, "INSERT INTO users(username,email,password_hash,role) VALUES($1,$2,$3,'admin') RETURNING id", username, email, string(hash)).Scan(&id); err != nil {
		return fmt.Errorf("create bootstrap administrator: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO arenas(slug,name,description,game_type,max_players,max_ticks,config_json,is_active)
		VALUES('battle-royale-5p','Battle Royale 5-Player Arena','Five distinct admitted agents per match','battle_royale_5p',5,10800,'{}',true)
		ON CONFLICT(slug) DO NOTHING`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs(user_id,action,target_type,target_id,details_json)
		VALUES($1,'SYSTEM_INIT','SYSTEM','BOOTSTRAP','{"message":"Explicit administrator bootstrap; no sample agents"}')`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
