package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Every test owns a fresh schema and migrations, not preexisting match IDs.
func fixtureRunner(t *testing.T) (*MatchRunner, context.Context, int) {
	t.Helper()
	url := os.Getenv("AGENTRIX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AGENTRIX_TEST_DATABASE_URL required for PostgreSQL integration")
	}
	ctx := context.Background()
	base, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	token, err := leaseToken()
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + token
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = base.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); base.Close() })
	for _, file := range []string{"000001_init_schema.up.sql", "000002_match_leases.up.sql", "000003_arena_freeze.up.sql", "000004_package_integrity.up.sql", "000005_tournament_rounds.up.sql", "000006_rules_snapshot.up.sql", "000007_replay_path_index.up.sql", "000008_runtime_provenance.up.sql"} {
		data, err := os.ReadFile(filepath.Join("../../migrations", file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	var arena int
	if err := pool.QueryRow(ctx, "INSERT INTO arenas(slug,name) VALUES('test','Test') RETURNING id").Scan(&arena); err != nil {
		t.Fatal(err)
	}
	return &MatchRunner{
		cfg:  &config.Config{RuntimeSHA256: strings.Repeat("0", 64)},
		pool: &db.Pool{Pool: pool},
	}, ctx, arena
}

func TestConcurrentClaimsAndFencing(t *testing.T) {
	r, ctx, arena := fixtureRunner(t)
	var id int
	if err := r.pool.QueryRow(ctx, "INSERT INTO matches(arena_id,seed) VALUES($1,1) RETURNING id", arena).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	leases := make(chan matchLease, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := r.claim(ctx, id)
			if err == nil {
				leases <- lease
			} else if err != pgx.ErrNoRows {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(leases)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if len(leases) != 1 {
		t.Fatalf("expected one owner, got %d", len(leases))
	}
	first := <-leases
	if err := r.recoverExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.claim(ctx, id); err != pgx.ErrNoRows {
		t.Fatalf("live lease recovered: %v", err)
	}
	if _, err := r.pool.Exec(ctx, "UPDATE matches SET lease_expires_at=now()-interval '1 second' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if err := r.recoverExpired(ctx); err != nil {
		t.Fatal(err)
	}
	second, err := r.claim(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if second.Owner == first.Owner || second.Attempt != 2 {
		t.Fatal("recovery did not fence previous attempt")
	}
	r.failMatch(context.WithValue(ctx, leaseKey{}, first.Owner), id, "stale worker error")
	var status, owner string
	if err := r.pool.QueryRow(ctx, "SELECT status,lease_owner FROM matches WHERE id=$1", id).Scan(&status, &owner); err != nil {
		t.Fatal(err)
	}
	if status != "running" || owner != second.Owner {
		t.Fatal("old worker modified new attempt")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := verifyLease(ctx, tx, id, first.Owner); err == nil {
		t.Fatal("stale worker can publish results")
	}
	if err := verifyLease(ctx, tx, id, second.Owner); err != nil {
		t.Fatal(err)
	}
}

func TestQueueClaimsDistinctRecords(t *testing.T) {
	r, ctx, arena := fixtureRunner(t)
	for i := 0; i < 10; i++ {
		if _, err := r.pool.Exec(ctx, "INSERT INTO matches(arena_id,seed) VALUES($1,$2)", arena, i); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan matchLease, 10)
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := r.claim(ctx, 0)
			if err != nil {
				errs <- err
			} else {
				results <- lease
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for lease := range results {
		if seen[lease.ID] {
			t.Fatal(fmt.Sprintf("duplicate claim %d", lease.ID))
		}
		seen[lease.ID] = true
	}
	if len(seen) != 10 {
		t.Fatalf("claimed %d of 10", len(seen))
	}
}

func TestUpgradeDoesNotImmediatelyReplayUnownedRunningMatch(t *testing.T) {
	r, ctx, arena := fixtureRunner(t)
	var id int
	if err := r.pool.QueryRow(ctx, "INSERT INTO matches(arena_id,seed,status) VALUES($1,1,'running') RETURNING id", arena).Scan(&id); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../migrations/000002_match_leases.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, string(data)); err != nil {
		t.Fatal(err)
	}
	if err := r.recoverExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	var leaseActive bool
	if err := r.pool.QueryRow(ctx, "SELECT status,lease_expires_at>now() FROM matches WHERE id=$1", id).Scan(&status, &leaseActive); err != nil {
		t.Fatal(err)
	}
	if status != "running" || !leaseActive {
		t.Fatal("upgrade replayed a potentially live old execution")
	}
}
