package api

import (
	"agentrix/backend/internal/auth"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRerunPreservesRulesAndRejectsUnsupportedArena(t *testing.T) {
	s, ctx, user, arena, agents := fixtureAPI(t)
	seed := int64(42)
	original, err := s.runner.ScheduleMatch(ctx, arena, agents, &seed)
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateToken(user, "judge", "admin", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	rerun := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/matches/%d/rerun", original), bytes.NewReader(nil))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := rerun(); w.Code != 409 {
		t.Fatalf("live job rerun: %d", w.Code)
	}
	if _, err := s.pool.Exec(ctx, "UPDATE matches SET status='failed' WHERE id=$1", original); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "UPDATE arenas SET max_ticks=1200,config_json='{}' WHERE id=$1", arena); err != nil {
		t.Fatal(err)
	}
	w := rerun()
	if w.Code != 202 {
		t.Fatalf("rerun %d: %s", w.Code, w.Body.String())
	}
	var reply struct {
		ID int `json:"new_match_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	var same bool
	if err := s.pool.QueryRow(ctx, "SELECT a.rules_json=b.rules_json AND a.rules_sha256=b.rules_sha256 AND a.runtime_sha256=b.runtime_sha256 FROM matches a,matches b WHERE a.id=$1 AND b.id=$2", original, reply.ID).Scan(&same); err != nil || !same {
		t.Fatalf("rules/runtime snapshot changed: %v", err)
	}
	s.cfg.RuntimeSHA256 = strings.Repeat("a", 64)
	if w := rerun(); w.Code != 409 {
		t.Fatalf("changed runtime rerun: %d %s", w.Code, w.Body.String())
	}
	s.cfg.RuntimeSHA256 = ""
	if _, err := s.pool.Exec(ctx, `UPDATE arenas SET config_json='{"grid_width":1000}' WHERE id=$1`, arena); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runner.ScheduleMatch(ctx, arena, agents, &seed); err == nil {
		t.Fatal("ignored geometry accepted")
	}
	if _, err := s.pool.Exec(ctx, "UPDATE arenas SET config_json='{}',max_players=3 WHERE id=$1", arena); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runner.ScheduleMatch(ctx, arena, agents, &seed); err == nil {
		t.Fatal("unsupported seats accepted")
	}
}
