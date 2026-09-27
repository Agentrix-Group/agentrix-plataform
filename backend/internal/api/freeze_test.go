package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/models"
)

func TestFreezeCoversLadderMatchesReplayAndCounts(t *testing.T) {
	s, ctx, userID, arenaID, agents := fixtureAPI(t)
	s.cfg.ReplaysDir = t.TempDir()
	token, err := auth.GenerateToken(userID, "judge", "admin", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	playerToken, err := auth.GenerateToken(userID, "player", "player", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	makeMatch := func(status string) int {
		var id int
		if err := s.pool.QueryRow(ctx, "INSERT INTO matches(arena_id,seed,status,finished_at) VALUES($1,1,$2,now()) RETURNING id", arenaID, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(s.cfg.ReplaysDir, fmt.Sprintf("match_%d.json", id))
		if err := os.WriteFile(file, []byte(`{"result":"fixture"}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.pool.Exec(ctx, "INSERT INTO replays(match_id,file_path,sha256,tick_count) VALUES($1,$2,'fixture',10)", id, file); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if _, err := s.pool.Exec(ctx, "INSERT INTO ladder_entries(arena_id,agent_version_id,matches_played,wins,kills,survival_ticks_total) VALUES($1,$2,1,1,2,100)", arenaID, agents[0]); err != nil {
		t.Fatal(err)
	}
	old := makeMatch("finished")
	running := makeMatch("running")
	ladderPath := fmt.Sprintf("/api/v1/arenas/%d/ladder", arenaID)
	freezePath := fmt.Sprintf("/api/v1/arenas/%d/freeze", arenaID)
	before := request("GET", ladderPath, "", "")
	if before.Code != 200 {
		t.Fatal(before.Body.String())
	}
	for _, who := range []string{"", playerToken} {
		if w := request("POST", freezePath, `{"frozen":true}`, who); w.Code == 200 {
			t.Fatal("non-admin can freeze")
		}
	}
	if w := request("POST", freezePath, `{"frozen":true}`, token); w.Code != 200 {
		t.Fatalf("freeze %d %s", w.Code, w.Body.String())
	}
	// Reconstruct the API to verify that visibility is persisted in PostgreSQL.
	s = NewServer(s.cfg, s.pool, s.runner, s.matchmaker)
	metadata := request("GET", fmt.Sprintf("/api/v1/arenas/%d", arenaID), "", "")
	var frozenArena models.Arena
	if err := json.Unmarshal(metadata.Body.Bytes(), &frozenArena); err != nil {
		t.Fatal(err)
	}
	if metadata.Code != 200 || !frozenArena.Frozen {
		t.Fatal("arena metadata does not expose freeze")
	}
	allArenas := request("GET", "/api/v1/arenas", "", "")
	var arenas []models.Arena
	if err := json.Unmarshal(allArenas.Body.Bytes(), &arenas); err != nil {
		t.Fatal(err)
	}
	if allArenas.Code != 200 || len(arenas) != 1 || !arenas[0].Frozen {
		t.Fatal("arena list lost freeze state")
	}
	newMatch := makeMatch("finished")
	if _, err := s.pool.Exec(ctx, "UPDATE matches SET status='finished' WHERE id=$1", running); err != nil {
		t.Fatal(err)
	}
	pending := makeMatch("running")
	var otherArena, otherMatch int
	if err := s.pool.QueryRow(ctx, "INSERT INTO arenas(slug,name) VALUES('other','Other') RETURNING id").Scan(&otherArena); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, "INSERT INTO matches(arena_id,seed,status) VALUES($1,1,'running') RETURNING id", otherArena).Scan(&otherMatch); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "UPDATE ladder_entries SET display_rating=1999,matches_played=2,kills=99,updated_at=now() WHERE arena_id=$1", arenaID); err != nil {
		t.Fatal(err)
	}
	// Idempotence: another freeze request cannot reveal new results.
	if w := request("POST", freezePath, `{"frozen":true}`, token); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var baseline []models.LadderEntry
	if err := json.Unmarshal(before.Body.Bytes(), &baseline); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"", playerToken} {
		w := request("GET", ladderPath, "", who)
		var actual []models.LadderEntry
		if err := json.Unmarshal(w.Body.Bytes(), &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, baseline) || w.Header().Get("X-Agentrix-Frozen") != "true" {
			t.Fatal("frozen ladder changed")
		}
		list := request("GET", "/api/v1/matches", "", who)
		var matches []models.Match
		if err := json.Unmarshal(list.Body.Bytes(), &matches); err != nil {
			t.Fatal(err)
		}
		if len(matches) != 2 || matches[0].ID != otherMatch || matches[1].ID != old {
			t.Fatalf("hidden match leaked: %s", list.Body.String())
		}
		list = request("GET", fmt.Sprintf("/api/v1/matches?limit=1&before_id=%d", otherMatch), "", who)
		if err := json.Unmarshal(list.Body.Bytes(), &matches); err != nil {
			t.Fatal(err)
		}
		if list.Code != 200 || len(matches) != 1 || matches[0].ID != old {
			t.Fatalf("cursor bypassed freeze: %s", list.Body.String())
		}
		for _, id := range []int{newMatch, running, pending} {
			for _, suffix := range []string{"", "/replay", "/download"} {
				w := request("GET", fmt.Sprintf("/api/v1/matches/%d%s", id, suffix), "", who)
				if w.Code != 404 {
					t.Fatalf("hidden route leaked %d%s: %d", id, suffix, w.Code)
				}
			}
		}
		count := request("GET", "/api/v1/system/status", "", who)
		var status models.SystemStatusResponse
		if err := json.Unmarshal(count.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.RunningMatches != 1 {
			t.Fatal("hidden count leaked")
		}
	}
	for _, suffix := range []string{"", "/replay", "/download"} {
		if w := request("GET", fmt.Sprintf("/api/v1/matches/%d%s", old, suffix), "", ""); w.Code != 200 {
			t.Fatalf("prefreeze result unavailable %s", suffix)
		}
		if w := request("GET", fmt.Sprintf("/api/v1/matches/%d%s", newMatch, suffix), "", token); w.Code != 200 {
			t.Fatalf("jury result unavailable %s", suffix)
		}
	}
	admin := request("GET", ladderPath, "", token)
	var current []models.LadderEntry
	if err := json.Unmarshal(admin.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current[0].DisplayRating != 1999 {
		t.Fatal("jury cannot see current ladder")
	}
	if w := request("POST", freezePath, `{"frozen":false}`, token); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	public := request("GET", ladderPath, "", "")
	var unfrozen []models.LadderEntry
	if err := json.Unmarshal(public.Body.Bytes(), &unfrozen); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unfrozen, current) {
		t.Fatal("unfreeze did not publish current ladder")
	}
	for _, suffix := range []string{"", "/replay", "/download"} {
		if w := request("GET", fmt.Sprintf("/api/v1/matches/%d%s", newMatch, suffix), "", ""); w.Code != 200 {
			t.Fatal("unfreeze did not release match")
		}
	}
}
