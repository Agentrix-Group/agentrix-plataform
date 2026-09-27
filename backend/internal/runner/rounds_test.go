package runner

import (
	"agentrix/backend/internal/tournament"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRoundIdempotencyAndBoundedMaterialization(t *testing.T) {
	r, ctx, arena := fixtureRunner(t)
	var owner int
	if err := r.pool.QueryRow(ctx, "INSERT INTO users(username,email,password_hash) VALUES('owner','owner@test','unused') RETURNING id").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	var roster []int
	for i := 0; i < 6; i++ {
		var team, agent int
		if err := r.pool.QueryRow(ctx, "INSERT INTO teams(name,owner_id) VALUES($1,$2) RETURNING id", fmt.Sprintf("team%d", i), owner).Scan(&team); err != nil {
			t.Fatal(err)
		}
		if err := r.pool.QueryRow(ctx, `INSERT INTO agent_versions(team_id,arena_id,name,artifact_path,sha256,artifact_sha256) VALUES($1,$2,$3,'fixture',repeat('a',64),repeat('b',64)) RETURNING id`, team, arena, fmt.Sprintf("bot%d", i)).Scan(&agent); err != nil {
			t.Fatal(err)
		}
		roster = append(roster, agent)
	}
	seed := int64(42)
	var wg sync.WaitGroup
	results := make(chan RoundReceipt, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := r.ScheduleRound(ctx, arena, "same", roster, &seed)
			if e != nil {
				errs <- e
			} else {
				results <- v
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	var roundID int
	for v := range results {
		if roundID != 0 && v.ID != roundID {
			t.Fatal("duplicated round")
		}
		roundID = v.ID
		if v.Total != 30 {
			t.Fatal(v)
		}
	}
	other := int64(43)
	var originalRules, originalHash, originalRuntime string
	if err := r.pool.QueryRow(ctx, "SELECT rules_json,rules_sha256,runtime_sha256 FROM tournament_rounds WHERE id=$1", roundID).Scan(&originalRules, &originalHash, &originalRuntime); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, "UPDATE arenas SET max_ticks=1200,config_json='{}' WHERE id=$1", arena); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ScheduleRound(ctx, arena, "same", roster, &other); !errors.Is(err, ErrRoundConflict) {
		t.Fatalf("conflict: %v", err)
	}
	r.cfg.RuntimeSHA256 = strings.Repeat("a", 64)
	if _, err := r.ScheduleRound(ctx, arena, "same", roster, &seed); !errors.Is(err, ErrRoundConflict) {
		t.Fatalf("changed runtime must conflict with idempotent round: %v", err)
	}
	r.cfg.RuntimeSHA256 = originalRuntime
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := r.FillRoundQueue(ctx); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	var count int
	if e := r.pool.QueryRow(ctx, "SELECT count(*) FROM matches WHERE round_id=$1", roundID).Scan(&count); e != nil || count != 25 {
		t.Fatalf("queue count %d: %v", count, e)
	}
	if _, e := r.pool.Exec(ctx, "UPDATE matches SET status='finished' WHERE round_id=$1", roundID); e != nil {
		t.Fatal(e)
	}
	if e := r.FillRoundQueue(ctx); e != nil {
		t.Fatal(e)
	}
	plan, _ := tournament.New(roster, 42)
	var differing int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM matches WHERE round_id=$1 AND (rules_json IS DISTINCT FROM $2 OR rules_sha256 IS DISTINCT FROM $3 OR runtime_sha256 IS DISTINCT FROM $4)", roundID, originalRules, originalHash, originalRuntime).Scan(&differing); err != nil || differing != 0 {
		t.Fatalf("round snapshot changed: %d %v", differing, err)
	}
	rows, e := r.pool.Query(ctx, `SELECT m.round_match_index,m.seed,mp.seat,mp.agent_version_id FROM matches m JOIN match_participants mp ON mp.match_id=m.id WHERE m.round_id=$1 ORDER BY m.round_match_index,mp.seat`, roundID)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for rows.Next() {
		var index, seat, agent int
		var actualSeed int64
		if e := rows.Scan(&index, &actualSeed, &seat, &agent); e != nil {
			t.Fatal(e)
		}
		m, e := plan.At(index)
		if e != nil || m.Agents[seat] != agent || int64(m.Seed) != actualSeed {
			t.Fatal("persisted plan mismatch")
		}
		n++
	}
	e = rows.Err()
	rows.Close()
	if e != nil || n != 150 {
		t.Fatalf("participants %d: %v", n, e)
	}
	if _, e := r.pool.Exec(ctx, "UPDATE matches SET status='finished' WHERE round_id=$1", roundID); e != nil {
		t.Fatal(e)
	}
	if e := r.FillRoundQueue(ctx); e != nil {
		t.Fatal(e)
	}
	var status string
	if e := r.pool.QueryRow(ctx, "SELECT status FROM tournament_rounds WHERE id=$1", roundID).Scan(&status); e != nil || status != "finished" {
		t.Fatalf("status %s: %v", status, e)
	}
}
