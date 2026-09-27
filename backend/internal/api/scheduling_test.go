package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/matchmaker"
	"agentrix/backend/internal/runner"
	"github.com/jackc/pgx/v5/pgxpool"
)

func fixtureAPI(t *testing.T) (*Server, context.Context, int, int, []int) {
	t.Helper()
	url := os.Getenv("AGENTRIX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("PostgreSQL integration requires AGENTRIX_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	base, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var tokenBytes [16]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(tokenBytes[:])
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
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
	var userID, arenaID int
	if err := pool.QueryRow(ctx, "INSERT INTO users(username,email,password_hash,role) VALUES('judge','judge@test','unused','admin') RETURNING id").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO arenas(slug,name) VALUES('test','Test') RETURNING id").Scan(&arenaID); err != nil {
		t.Fatal(err)
	}
	var agents []int
	for i := 0; i < 5; i++ {
		var teamID, agentID int
		if err := pool.QueryRow(ctx, "INSERT INTO teams(name,owner_id) VALUES($1,$2) RETURNING id", fmt.Sprintf("team-%d", i), userID).Scan(&teamID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, "INSERT INTO agent_versions(team_id,arena_id,name,artifact_path,sha256) VALUES($1,$2,'Fixture','unused','unused') RETURNING id", teamID, arenaID).Scan(&agentID); err != nil {
			t.Fatal(err)
		}
		agents = append(agents, agentID)
	}
	serverCfg := &config.Config{JWTSecret: "test-secret"}
	dbPool := &db.Pool{Pool: pool}
	matchRunner := runner.NewMatchRunner(serverCfg, dbPool)
	s := NewServer(serverCfg, dbPool, matchRunner, matchmaker.NewMatchmaker(serverCfg, dbPool, matchRunner))
	return s, ctx, userID, arenaID, agents
}

func TestAdminCanScheduleMatchOnFreshDatabase(t *testing.T) {
	s, ctx, userID, arenaID, agents := fixtureAPI(t)
	serverCfg, pool := s.cfg, s.pool
	token, err := auth.GenerateToken(userID, "judge", "admin", serverCfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]interface{}{"arena_id": arenaID, "agent_version_ids": agents, "seed": 123})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/matches/trigger", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	if w.Code != 202 {
		t.Fatalf("schedule: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		MatchID int `json:"match_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var count int
	var status string
	var seed int64
	if err := pool.QueryRow(ctx, "SELECT status,seed FROM matches WHERE id=$1", result.MatchID).Scan(&status, &seed); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(DISTINCT agent_version_id) FROM match_participants WHERE match_id=$1", result.MatchID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if status != "scheduled" || seed != 123 || count != 5 {
		t.Fatal("scheduled match does not preserve requested participants/seed")
	}
}
