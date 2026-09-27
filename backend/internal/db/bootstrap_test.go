package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func TestBootstrapIsExplicitConcurrentAndNeverSeedsAgents(t *testing.T) {
	url := os.Getenv("AGENTRIX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires temporary PostgreSQL")
	}
	ctx := context.Background()
	base, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "bootstrap_" + hex.EncodeToString(nonce[:])
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); base.Close() })
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	raw, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(raw.Close)
	p := &Pool{raw}
	migration, err := os.ReadFile("../../migrations/000001_init_schema.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if err := p.Bootstrap(ctx, "", "", ""); err != nil {
		t.Fatal(err)
	}
	count := func(table string, want int) {
		t.Helper()
		var n int
		if err := p.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Fatalf("%s: %d want %d", table, n, want)
		}
	}
	count("users", 0)
	count("arenas", 0)
	const password = "private-fixture-password-2026"
	if _, err := p.Exec(ctx, "INSERT INTO users(username,email,password_hash,role) VALUES('collision','player@test','unused','player')"); err != nil {
		t.Fatal(err)
	}
	if err := p.Bootstrap(ctx, "collision", "judge@test", password); err == nil {
		t.Fatal("bootstrap elevated a conflicting player account")
	}
	count("users", 1)
	count("arenas", 0)
	count("audit_logs", 0)
	var role string
	if err := p.QueryRow(ctx, "SELECT role FROM users WHERE username='collision'").Scan(&role); err != nil || role != "player" {
		t.Fatalf("conflicting account changed: %s %v", role, err)
	}
	if _, err := p.Exec(ctx, "DELETE FROM users WHERE username='collision'"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- p.Bootstrap(ctx, "judge", "judge@test", password) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	for table, want := range map[string]int{"users": 1, "arenas": 1, "audit_logs": 1, "teams": 0, "agent_versions": 0, "ladder_entries": 0} {
		count(table, want)
	}
	var before, after string
	if err := p.QueryRow(ctx, "SELECT password_hash FROM users WHERE username='judge'").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(before), []byte(password)); err != nil {
		t.Fatal(err)
	}
	if err := p.Bootstrap(ctx, "different", "different@test", "different-private-password"); err != nil {
		t.Fatal(err)
	}
	count("users", 1)
	if err := p.QueryRow(ctx, "SELECT password_hash FROM users WHERE username='judge'").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("bootstrap overwrote existing credentials")
	}
}
